package latexencode

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/phfaist/pylatexenc"
	"github.com/phfaist/pylatexenc/internal/pyrt"
)

var templateEscapes = map[rune]rune{'a': '\a', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t', 'v': '\v', '\\': '\\'}

type templateError struct{ err error }

type templateTokenizer struct {
	s     []rune
	index int
	next  string
}

func (t *templateTokenizer) advance() {
	if t.index >= len(t.s) {
		t.next = ""
		return
	}
	idx := t.index
	ch := string(t.s[idx])
	if t.s[idx] == '\\' {
		idx++
		if idx >= len(t.s) {
			panic(templateError{t.errorAt("bad escape (end of pattern)", len(t.s)-1)})
		}
		ch += string(t.s[idx])
	}
	t.index = idx + 1
	t.next = ch
}

func (t *templateTokenizer) get() string {
	this := t.next
	t.advance()
	return this
}

func (t *templateTokenizer) errorAt(msg string, pos int) error {
	msg = fmt.Sprintf("%s at position %d", msg, pos)
	if strings.ContainsRune(string(t.s), '\n') {
		before := t.s[:pos]
		lineno, lastNL := 1, -1
		for i, c := range before {
			if c == '\n' {
				lineno++
				lastNL = i
			}
		}
		msg = fmt.Sprintf("%s (line %d, column %d)", msg, lineno, pos-lastNL)
	}
	return &pylatexenc.BuiltinError{Class: "error", Message: msg}
}

func (t *templateTokenizer) fail(msg string, offset int) {
	panic(templateError{t.errorAt(msg, t.index-len([]rune(t.next))-offset)})
}

func isIdentifier(s string) bool {
	for i, c := range s {
		if c == '_' || unicode.IsLetter(c) || unicode.Is(unicode.Nl, c) {
			continue
		}
		if i > 0 && (unicode.IsDigit(c) || unicode.In(c, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc)) {
			continue
		}
		return false
	}
	return s != ""
}

func isASCIIDecimal(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

func isOct(s string) bool { return len(s) == 1 && s[0] >= '0' && s[0] <= '7' }

func isDigit(s string) bool { return len(s) == 1 && s[0] >= '0' && s[0] <= '9' }

// expandTemplate expands repl for a match, as Python's Match.expand() does: groups that did
// not take part in the match expand to "".
func expandTemplate(repl string, rx *regexp.Regexp, groups []string) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			te, ok := r.(templateError)
			if !ok {
				panic(r)
			}
			out, err = "", te.err
		}
	}()
	t := &templateTokenizer{s: []rune(repl)}
	t.advance()
	var b strings.Builder
	addGroup := func(index, offset int) {
		if index > len(groups)-1 {
			t.fail(fmt.Sprintf("invalid group reference %d", index), offset)
		}
		b.WriteString(groups[index])
	}
	for {
		this := t.get()
		if this == "" {
			break
		}
		r := []rune(this)
		if r[0] != '\\' {
			b.WriteString(this)
			continue
		}
		c := string(r[1])
		switch {
		case c == "g":
			if t.next != "<" {
				t.fail("missing <", 0)
			}
			t.advance()
			name := ""
			for {
				ch := t.next
				t.advance()
				if ch == "" {
					if name == "" {
						t.fail("missing group name", 0)
					}
					t.fail("missing >, unterminated name", len([]rune(name)))
				}
				if ch == ">" {
					if name == "" {
						t.fail("missing group name", 1)
					}
					break
				}
				name += ch
			}
			var index int
			if isASCIIDecimal(name) {
				n, perr := strconv.Atoi(name)
				if perr != nil || n >= 1<<30-1 {
					t.fail("invalid group reference "+strings.TrimLeft(name, "0"), len([]rune(name))+1)
				}
				index = n
			} else {
				if !isIdentifier(name) {
					t.fail("bad character in group name "+pyrt.Repr(name), len([]rune(name))+1)
				}
				if index = rx.SubexpIndex(name); index < 0 {
					return "", &pylatexenc.BuiltinError{Class: "IndexError", Message: "unknown group name " + pyrt.Repr(name)}
				}
			}
			addGroup(index, len([]rune(name))+1)
		case c == "0":
			digits := "0"
			if isOct(t.next) {
				digits += t.get()
				if isOct(t.next) {
					digits += t.get()
				}
			}
			v, _ := strconv.ParseInt(digits, 8, 32)
			b.WriteRune(rune(v & 0xff))
		case isDigit(c):
			num := c
			isOctal := false
			if isDigit(t.next) {
				num += t.get()
				if isOct(c) && isOct(num[1:2]) && isOct(t.next) {
					num += t.get()
					isOctal = true
					v, _ := strconv.ParseInt(num, 8, 32)
					if v > 0o377 {
						t.fail(`octal escape value \`+num+` outside of range 0-0o377`, len(num)+1)
					}
					b.WriteRune(rune(v))
				}
			}
			if !isOctal {
				index, _ := strconv.Atoi(num)
				addGroup(index, len(num))
			}
		default:
			if e, ok := templateEscapes[r[1]]; ok {
				b.WriteRune(e)
			} else if (r[1] >= 'a' && r[1] <= 'z') || (r[1] >= 'A' && r[1] <= 'Z') {
				t.fail("bad escape "+this, 2)
			} else {
				b.WriteString(this)
			}
		}
	}
	return b.String(), nil
}
