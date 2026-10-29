// Package latexencode converts Unicode text to LaTeX, as pylatexenc.latexencode 2.11 does.
package latexencode

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/phfaist/pylatexenc"
	"github.com/phfaist/pylatexenc/internal/pyrt"
)

type uni2latexEntry struct {
	cp    rune
	latex string
}

var (
	builtinOnce sync.Once
	defaultsMap map[rune]string
	xmlMap      map[rune]string
)

func loadBuiltins() {
	builtinOnce.Do(func() {
		defaultsMap = make(map[rune]string, len(uni2latexDefaults))
		for _, e := range uni2latexDefaults {
			defaultsMap[e.cp] = e.latex
		}
		xmlMap = make(map[rune]string, len(uni2latexXML))
		for _, e := range uni2latexXML {
			xmlMap[e.cp] = e.latex
		}
	})
}

func copyMap(m map[rune]string) map[rune]string {
	c := make(map[rune]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// GetBuiltinUni2LatexDict returns a copy of the built-in "defaults" character map.
func GetBuiltinUni2LatexDict() map[rune]string {
	loadBuiltins()
	return copyMap(defaultsMap)
}

// RuleType is the kind of a conversion rule.
type RuleType int

const (
	RuleDict     RuleType = 0
	RuleRegex    RuleType = 1
	RuleCallable RuleType = 2
)

// RuleFunc is a callable rule. It looks at the code points of the (NFC-normalized) string
// at code point index pos; it returns ok=false to pass, or the number of code points it
// consumes and their replacement.
type RuleFunc func(s []rune, pos int, enc *UnicodeToLatexEncoder) (consumed int, repl string, ok bool, err error)

// RegexReplacement is one (regex, replacement) pair of a regex rule. The regexp is matched
// at the current position. Repl is a template expanded as Python's Match.expand() expands
// it; ReplFunc, when set, computes the replacement from the submatches instead.
type RegexReplacement struct {
	Regexp   *regexp.Regexp
	Repl     string
	ReplFunc func(groups []string) string
}

// UnicodeToLatexConversionRule is a conversion rule: a dictionary of code points
// (RuleDict), a list of regex replacements (RuleRegex) or a callable (RuleCallable).
// ReplacementLatexProtection, or ProtectionFunc, overrides the encoder's protection for the
// replacements of this rule; "" means no override.
type UnicodeToLatexConversionRule struct {
	RuleType                   RuleType
	Dict                       map[rune]string
	Regexes                    []RegexReplacement
	Callable                   RuleFunc
	ReplacementLatexProtection string
	ProtectionFunc             func(repl string) string
}

// ConversionRule is an entry of Options.ConversionRules: a BuiltinRules name or a
// *UnicodeToLatexConversionRule.
type ConversionRule interface{ isConversionRule() }

// BuiltinRules names a built-in rule set: "defaults" or "unicode-xml".
type BuiltinRules string

func (BuiltinRules) isConversionRule() {}

func (*UnicodeToLatexConversionRule) isConversionRule() {}

// GetBuiltinConversionRules returns the rules of a built-in rule set.
func GetBuiltinConversionRules(name string) ([]*UnicodeToLatexConversionRule, error) {
	loadBuiltins()
	switch name {
	case "defaults":
		return []*UnicodeToLatexConversionRule{{RuleType: RuleDict, Dict: defaultsMap}}, nil
	case "unicode-xml":
		return []*UnicodeToLatexConversionRule{{RuleType: RuleDict, Dict: xmlMap}}, nil
	}
	return nil, valueError("Unknown builtin rule set: " + name)
}

// Options are the UnicodeToLatexEncoder options. The zero value means no rules and invalid
// policy names, as the equivalent Python arguments would; DefaultOptions has the Python
// defaults. UnknownCharFunc, when set, replaces UnknownCharPolicy, and ProtectionFunc
// replaces ReplacementLatexProtection.
type Options struct {
	NonASCIIOnly               bool
	ConversionRules            []ConversionRule
	ReplacementLatexProtection string
	ProtectionFunc             func(repl string) string
	UnknownCharPolicy          string
	UnknownCharFunc            func(ch rune) (string, error)
	UnknownCharWarning         bool
}

// DefaultOptions are the Python defaults.
func DefaultOptions() Options {
	return Options{ConversionRules: []ConversionRule{BuiltinRules("defaults")},
		ReplacementLatexProtection: "braces", UnknownCharPolicy: "keep", UnknownCharWarning: true}
}

// UnicodeToLatexEncoder encodes Unicode strings to LaTeX.
type UnicodeToLatexEncoder struct {
	Options
	rules       []*UnicodeToLatexConversionRule
	protect     func(string) string
	unknownChar func(rune) (string, error)
}

func valueError(msg string) error { return &pylatexenc.BuiltinError{Class: "ValueError", Message: msg} }

func typeError(msg string) error { return &pylatexenc.BuiltinError{Class: "TypeError", Message: msg} }

// NewUnicodeToLatexEncoder checks the options in the order the Python constructor does:
// the conversion rules, then the unknown-character policy, then the protection.
func NewUnicodeToLatexEncoder(opts Options) (*UnicodeToLatexEncoder, error) {
	e := &UnicodeToLatexEncoder{Options: opts}
	if err := e.init(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *UnicodeToLatexEncoder) init() error {
	for _, r := range e.ConversionRules {
		var expanded []*UnicodeToLatexConversionRule
		switch x := r.(type) {
		case BuiltinRules:
			rs, err := GetBuiltinConversionRules(string(x))
			if err != nil {
				return err
			}
			expanded = rs
		case *UnicodeToLatexConversionRule:
			expanded = []*UnicodeToLatexConversionRule{x}
		}
		for _, rule := range expanded {
			switch rule.RuleType {
			case RuleDict, RuleRegex, RuleCallable:
			default:
				return typeError(fmt.Sprintf("Invalid rule type: %d", int(rule.RuleType)))
			}
			e.rules = append(e.rules, rule)
		}
	}
	if e.UnknownCharFunc != nil {
		e.unknownChar = e.UnknownCharFunc
	} else {
		fn, ok := unknownCharPolicies[strings.ReplaceAll(e.UnknownCharPolicy, "-", "_")]
		if !ok {
			return valueError("Invalid unknown_char_policy: " + e.UnknownCharPolicy)
		}
		e.unknownChar = fn
	}
	protect, err := protectionFn(e.ReplacementLatexProtection, e.ProtectionFunc)
	if err != nil {
		return err
	}
	e.protect = protect
	return nil
}

var unknownCharPolicies = map[string]func(rune) (string, error){
	"keep":    func(ch rune) (string, error) { return string(ch), nil },
	"replace": func(ch rune) (string, error) { return `{\bfseries ?}`, nil },
	"ignore":  func(ch rune) (string, error) { return "", nil },
	"fail": func(ch rune) (string, error) {
		return "", valueError(fmt.Sprintf("No known latex representation for character: U+%04X - ‘%s’", ch, string(ch)))
	},
	"unihex": func(ch rune) (string, error) {
		return fmt.Sprintf(`\ensuremath{\langle}\texttt{U+%04X}\ensuremath{\rangle}`, ch), nil
	},
}

func lastMacroIsAlpha(repl string) bool {
	k := strings.LastIndexByte(repl, '\\')
	return k >= 0 && pyrt.AllAlpha(repl[k+1:])
}

var protections = map[string]func(string) string{
	"none": func(r string) string { return r },
	"braces": func(r string) string {
		if lastMacroIsAlpha(r) {
			return "{" + r + "}"
		}
		return r
	},
	"braces_almost_all": func(r string) string {
		if strings.HasPrefix(r, `\`) {
			return "{" + r + "}"
		}
		return r
	},
	"braces_all": func(r string) string { return "{" + r + "}" },
	"braces_after_macro": func(r string) string {
		if lastMacroIsAlpha(r) {
			return r + "{}"
		}
		return r
	},
}

func protectionFn(name string, fn func(string) string) (func(string) string, error) {
	if fn != nil {
		return fn, nil
	}
	p, ok := protections[strings.ReplaceAll(name, "-", "_")]
	if !ok {
		return nil, valueError("Invalid replacement_latex_protection: " + name)
	}
	return p, nil
}

func (e *UnicodeToLatexEncoder) applyRule(rule *UnicodeToLatexConversionRule, s []rune, pos int) (consumed int, repl string, ok bool, err error) {
	switch rule.RuleType {
	case RuleDict:
		r, found := rule.Dict[s[pos]]
		return 1, r, found, nil
	case RuleRegex:
		rest := string(s[pos:])
		for _, rx := range rule.Regexes {
			loc := anchoredMatch(rx.Regexp, rest)
			if loc == nil {
				continue
			}
			groups := make([]string, len(loc)/2)
			for g := range groups {
				if loc[2*g] >= 0 {
					groups[g] = rest[loc[2*g]:loc[2*g+1]]
				}
			}
			var replstr string
			if rx.ReplFunc != nil {
				replstr = rx.ReplFunc(groups)
			} else if replstr, err = expandTemplate(rx.Repl, rx.Regexp, groups); err != nil {
				return 0, "", false, err
			}
			return utf8.RuneCountInString(rest[:loc[1]]), replstr, true, nil
		}
		return 0, "", false, nil
	case RuleCallable:
		if rule.Callable == nil {
			return 0, "", false, typeError("'NoneType' object is not callable")
		}
		return rule.Callable(s, pos, e)
	}
	return 0, "", false, nil
}

var anchoredCache sync.Map

// anchoredMatch matches rx at the start of s only, as Python's pattern.match(s, pos) does.
func anchoredMatch(rx *regexp.Regexp, s string) []int {
	v, ok := anchoredCache.Load(rx)
	if !ok {
		v, _ = anchoredCache.LoadOrStore(rx, regexp.MustCompile(`^(?:`+rx.String()+`)`))
	}
	return v.(*regexp.Regexp).FindStringSubmatchIndex(s)
}

// UnicodeToLatex encodes s.
func (e *UnicodeToLatexEncoder) UnicodeToLatex(s string) (string, error) {
	r := []rune(pyrt.NFC(s))
	var b strings.Builder
	pos := 0
	for pos < len(r) {
		if e.NonASCIIOnly && r[pos] < 127 {
			b.WriteRune(r[pos])
			pos++
			continue
		}
		applied := false
		for _, rule := range e.rules {
			consumed, repl, ok, err := e.applyRule(rule, r, pos)
			if err != nil {
				return "", err
			}
			if !ok {
				continue
			}
			protect := e.protect
			if rule.ProtectionFunc != nil || rule.ReplacementLatexProtection != "" {
				if protect, err = protectionFn(rule.ReplacementLatexProtection, rule.ProtectionFunc); err != nil {
					return "", err
				}
			}
			b.WriteString(protect(repl))
			pos += consumed
			applied = true
			break
		}
		if applied {
			continue
		}
		ch := r[pos]
		if (ch >= 32 && ch <= 127) || ch == '\n' || ch == '\r' || ch == '\t' {
			b.WriteRune(ch)
			pos++
			continue
		}
		repl, err := e.unknownChar(ch)
		if err != nil {
			return "", err
		}
		b.WriteString(repl)
		pos++
	}
	return b.String(), nil
}

// UnicodeToLatex is the module-level unicode_to_latex(): it encodes s with an encoder built
// from opts.
func UnicodeToLatex(s string, opts Options) (string, error) {
	e, err := NewUnicodeToLatexEncoder(opts)
	if err != nil {
		return "", err
	}
	return e.UnicodeToLatex(s)
}

// Utf8ToLatex is the deprecated utf8tolatex() function.
func Utf8ToLatex(s string, nonASCIIOnly, brackets, substituteBadChars, failBadChars bool) (string, error) {
	loadBuiltins()
	s = pyrt.NFC(s)
	var b strings.Builder
	for _, ch := range s {
		if nonASCIIOnly && ch < 127 {
			b.WriteRune(ch)
			continue
		}
		if lch, ok := defaultsMap[ch]; ok {
			if brackets && strings.HasPrefix(lch, `\`) {
				b.WriteString("{" + lch + "}")
			} else {
				b.WriteString(lch)
			}
		} else if (ch >= 32 && ch <= 127) || ch == '\n' || ch == '\r' || ch == '\t' {
			b.WriteRune(ch)
		} else {
			msg := fmt.Sprintf("Character cannot be encoded into LaTeX: U+%04X - `%s'", ch, string(ch))
			if failBadChars {
				return "", valueError(msg)
			}
			if substituteBadChars {
				b.WriteString(`{\bfseries ?}`)
			} else {
				b.WriteRune(ch)
			}
		}
	}
	return b.String(), nil
}
