package pyrt

import (
	"fmt"
	"strings"

	"github.com/phfaist/pylatexenc"
)

// StrDict is a Python dict of str to str, which keeps insertion order.
type StrDict struct {
	keys   []string
	values map[string]string
}

// NewStrDict returns an empty dict.
func NewStrDict() *StrDict { return &StrDict{values: map[string]string{}} }

// Set is d[key] = value.
func (d *StrDict) Set(key, value string) {
	if _, ok := d.values[key]; !ok {
		d.keys = append(d.keys, key)
	}
	d.values[key] = value
}

// Repr is repr() of the dict.
func (d *StrDict) Repr() string {
	parts := make([]string, len(d.keys))
	for i, k := range d.keys {
		parts[i] = Repr(k) + ": " + Repr(d.values[k])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// PercentArgs is the right operand of str % args: a tuple of strings, or a dict when Dict
// is not nil.
type PercentArgs struct {
	Tuple []string
	Dict  *StrDict
}

type pyValue struct {
	str  string
	dict *StrDict
}

func (v pyValue) toStr() string {
	if v.dict != nil {
		return v.dict.Repr()
	}
	return v.str
}

func (v pyValue) toRepr(asciiOnly bool) string {
	if v.dict != nil {
		if asciiOnly {
			return asciiOfRepr(v.dict.Repr())
		}
		return v.dict.Repr()
	}
	if asciiOnly {
		return ASCII(v.str)
	}
	return Repr(v.str)
}

// asciiOfRepr escapes the non-ASCII characters left in a repr(), as ascii() does.
func asciiOfRepr(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r <= 0xffff:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	return b.String()
}

func typeError(msg string) error { return &pylatexenc.BuiltinError{Class: "TypeError", Message: msg} }

func valueError(msg string) error { return &pylatexenc.BuiltinError{Class: "ValueError", Message: msg} }

// Percent is format % args for a str format, as CPython 3.12's PyUnicode_Format computes it,
// for arguments that are strings or a dict of strings. Errors are *pylatexenc.BuiltinError
// values of class TypeError, ValueError or KeyError.
func Percent(format string, args PercentArgs) (string, error) {
	f := []rune(format)
	var (
		argsVal pyValue
		arglen  int
		argidx  int
		tuple   []string
		dict    = args.Dict
	)
	if dict == nil {
		tuple = args.Tuple
		arglen = len(tuple)
		argidx = 0
	} else {
		argsVal = pyValue{dict: dict}
		arglen = -1
		argidx = -2
	}
	nextArg := func() (pyValue, error) {
		if argidx < arglen {
			idx := argidx
			argidx++
			if arglen < 0 {
				return argsVal, nil
			}
			return pyValue{str: tuple[idx]}, nil
		}
		return pyValue{}, typeError("not enough arguments for format string")
	}

	var out strings.Builder
	n := len(f)
	pos := 0
	for pos < n {
		if f[pos] != '%' {
			out.WriteRune(f[pos])
			pos++
			continue
		}
		pos++
		if pos < n && f[pos] == '%' {
			out.WriteByte('%')
			pos++
			continue
		}
		at := func() rune {
			if pos < n {
				return f[pos]
			}
			return 0
		}
		if at() == '(' {
			if dict == nil {
				return "", typeError("format requires a mapping")
			}
			pos++
			keystart := pos
			pcount := 1
			for pcount > 0 && pos < n {
				if f[pos] == ')' {
					pcount--
				} else if f[pos] == '(' {
					pcount++
				}
				pos++
			}
			if pcount > 0 {
				return "", valueError("incomplete format key")
			}
			key := string(f[keystart : pos-1])
			v, ok := dict.values[key]
			if !ok {
				return "", &pylatexenc.BuiltinError{Class: "KeyError", Message: Repr(key)}
			}
			argsVal = pyValue{str: v}
			arglen = -1
			argidx = -2
		}
		ljust := false
		ch := rune(0)
		have := false
		read := func() bool {
			if pos < n {
				ch = f[pos]
				pos++
				return true
			}
			return false
		}
		for {
			if !read() {
				have = false
				break
			}
			have = true
			if ch == '-' {
				ljust = true
				continue
			}
			if ch == '+' || ch == ' ' || ch == '#' || ch == '0' {
				continue
			}
			break
		}
		width, prec := -1, -1
		if have && ch == '*' {
			v, err := nextArg()
			if err != nil {
				return "", err
			}
			_ = v
			return "", typeError("* wants int")
		} else if have && ch >= '0' && ch <= '9' {
			width = int(ch - '0')
			have = false
			for read() {
				have = true
				if ch < '0' || ch > '9' {
					break
				}
				have = false
				width = width*10 + int(ch-'0')
			}
		}
		if have && ch == '.' {
			prec = 0
			have = read()
			if have && ch == '*' {
				if _, err := nextArg(); err != nil {
					return "", err
				}
				return "", typeError("* wants int")
			} else if have && ch >= '0' && ch <= '9' {
				prec = int(ch - '0')
				have = false
				for read() {
					have = true
					if ch < '0' || ch > '9' {
						break
					}
					have = false
					prec = prec*10 + int(ch-'0')
				}
			}
		}
		if have && (ch == 'h' || ch == 'l' || ch == 'L') {
			have = read()
		}
		if !have {
			return "", valueError("incomplete format")
		}
		v, err := nextArg()
		if err != nil {
			return "", err
		}
		var s string
		switch ch {
		case 's':
			s = v.toStr()
		case 'r':
			s = v.toRepr(false)
		case 'a':
			s = v.toRepr(true)
		case 'i', 'd', 'u':
			return "", typeError(fmt.Sprintf("%%%c format: a real number is required, not %s", ch, v.typeName()))
		case 'o', 'x', 'X':
			return "", typeError(fmt.Sprintf("%%%c format: an integer is required, not %s", ch, v.typeName()))
		case 'e', 'E', 'f', 'F', 'g', 'G':
			return "", typeError("must be real number, not " + v.typeName())
		case 'c':
			if v.dict != nil || Len(v.str) != 1 {
				return "", typeError("%c requires int or char")
			}
			s = v.str
		default:
			c := '?'
			if ch >= 31 && ch <= 126 {
				c = ch
			}
			return "", valueError(fmt.Sprintf("unsupported format character '%c' (0x%x) at index %d", c, ch, pos-1))
		}
		runes := []rune(s)
		if (ch == 's' || ch == 'r' || ch == 'a') && prec >= 0 && len(runes) > prec {
			runes = runes[:prec]
		}
		pad := width - len(runes)
		if pad > 0 && !ljust {
			out.WriteString(strings.Repeat(" ", pad))
		}
		out.WriteString(string(runes))
		if pad > 0 && ljust {
			out.WriteString(strings.Repeat(" ", pad))
		}
		if dict != nil && argidx < arglen {
			return "", typeError("not all arguments converted during string formatting")
		}
	}
	if argidx < arglen && dict == nil {
		return "", typeError("not all arguments converted during string formatting")
	}
	return out.String(), nil
}

func (v pyValue) typeName() string {
	if v.dict != nil {
		return "dict"
	}
	return "str"
}

// HasPercentS reports whether re.search('(^|[^%])(%%)*%s', s) finds a match: some maximal
// run of '%' of odd length is directly followed by 's'.
func HasPercentS(s string) bool {
	r := []rune(s)
	for i := 0; i < len(r); {
		if r[i] != '%' {
			i++
			continue
		}
		j := i
		for j < len(r) && r[j] == '%' {
			j++
		}
		if (j-i)%2 == 1 && j < len(r) && r[j] == 's' {
			return true
		}
		i = j
	}
	return false
}
