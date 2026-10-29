package latexencode

import (
	"strings"

	"github.com/phfaist/pylatexenc/latexwalker"
)

// DefaultKeepLatexChars is the default set of characters at which
// PartialLatexToLatexEncoder keeps the LaTeX token that starts there.
const DefaultKeepLatexChars = `\${}^_`

// PartialLatexToLatexEncoder encodes text that already contains some LaTeX: a token that
// starts with one of KeepLatexChars is copied through unchanged.
type PartialLatexToLatexEncoder struct {
	*UnicodeToLatexEncoder
	KeepLatexChars string
}

// NewPartialLatexToLatexEncoder returns a partial encoder. Its rules are the token-keeping
// rule, with protection "none", followed by opts.ConversionRules; DefaultOptions() gives
// Python's default rules ["defaults"].
func NewPartialLatexToLatexEncoder(keepLatexChars string, opts Options) (*PartialLatexToLatexEncoder, error) {
	p := &PartialLatexToLatexEncoder{KeepLatexChars: keepLatexChars}
	keep := &UnicodeToLatexConversionRule{RuleType: RuleCallable, Callable: p.partialStep, ReplacementLatexProtection: "none"}
	opts.ConversionRules = append([]ConversionRule{keep}, opts.ConversionRules...)
	enc, err := NewUnicodeToLatexEncoder(opts)
	if err != nil {
		return nil, err
	}
	p.UnicodeToLatexEncoder = enc
	return p, nil
}

func (p *PartialLatexToLatexEncoder) partialStep(s []rune, pos int, _ *UnicodeToLatexEncoder) (int, string, bool, error) {
	if !strings.ContainsRune(p.KeepLatexChars, s[pos]) {
		return 0, "", false, nil
	}
	w := latexwalker.NewLatexWalker(string(s), latexwalker.Options{})
	tok, err := w.GetToken(pos, nil, false, nil)
	if err != nil {
		return 0, "", false, err
	}
	return tok.Pos + tok.Len - pos, tok.PreSpace + string(s[tok.Pos:tok.Pos+tok.Len]), true, nil
}
