package latexencode_test

import (
	"errors"
	"testing"

	"github.com/phfaist/pylatexenc"
	"github.com/phfaist/pylatexenc/latexencode"
)

func TestUnicodeToLatex(t *testing.T) {
	base := latexencode.DefaultOptions()
	base.UnknownCharWarning = false
	with := func(f func(*latexencode.Options)) latexencode.Options {
		o := base
		f(&o)
		return o
	}
	cases := []struct {
		name string
		opts latexencode.Options
		in   string
		want string
	}{
		{"defaults", base, "café ñ – α ≤ x", `caf\'e \~n {\textendash} \ensuremath{\alpha} \ensuremath{\leq} x`},
		{"unihex", with(func(o *latexencode.Options) { o.UnknownCharPolicy = "unihex" }), "snow \U0001F600",
			`snow \ensuremath{\langle}\texttt{U+1F600}\ensuremath{\rangle}`},
		{"braces-all", with(func(o *latexencode.Options) { o.ReplacementLatexProtection = "braces-all" }), "é ß", `{\'e} {\ss}`},
		{"non-ascii-only", with(func(o *latexencode.Options) { o.NonASCIIOnly = true }), "{a} & é", `{a} & \'e`},
	}
	for _, c := range cases {
		got, err := latexencode.UnicodeToLatex(c.in, c.opts)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
		} else if got != c.want {
			t.Errorf("%s: got %q; want %q", c.name, got, c.want)
		}
	}
}

func TestErrors(t *testing.T) {
	opts := latexencode.DefaultOptions()
	opts.UnknownCharPolicy = "fail"
	_, err := latexencode.UnicodeToLatex("x\U0001F600", opts)
	var be *pylatexenc.BuiltinError
	if !errors.As(err, &be) || be.Class != "ValueError" ||
		be.Message != "No known latex representation for character: U+1F600 - ‘😀’" {
		t.Errorf("fail policy: err = %v", err)
	}
	opts.UnknownCharPolicy = "nope"
	if _, err := latexencode.NewUnicodeToLatexEncoder(opts); err == nil ||
		err.Error() != "ValueError: Invalid unknown_char_policy: nope" {
		t.Errorf("bad policy: err = %v", err)
	}
}

func TestPartialEncoder(t *testing.T) {
	p, err := latexencode.NewPartialLatexToLatexEncoder(latexencode.DefaultKeepLatexChars, latexencode.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.UnicodeToLatex(`\textbf{é} $x$ ü & 50%`)
	if err != nil {
		t.Fatal(err)
	}
	if want := `\textbf{\'e} $x$ \"u \& 50\%`; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestUtf8ToLatex(t *testing.T) {
	got, err := latexencode.Utf8ToLatex("é\U0001F600", false, true, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{\'e}{\bfseries ?}`; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}
