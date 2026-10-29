package latex2text_test

import (
	"errors"
	"testing"

	"github.com/phfaist/pylatexenc"
	"github.com/phfaist/pylatexenc/latex2text"
	"github.com/phfaist/pylatexenc/latexwalker"
)

func convert(t *testing.T, opts latex2text.Options, s string) string {
	t.Helper()
	got, err := latex2text.LatexToText(s, opts, latexwalker.DefaultOptions())
	if err != nil {
		t.Fatalf("LatexToText(%q): %v", s, err)
	}
	return got
}

func TestConversions(t *testing.T) {
	fill := latex2text.DefaultOptions()
	fill.FillText = 24
	delims := latex2text.DefaultOptions()
	delims.MathMode = "with-delimiters"
	cases := []struct {
		name string
		opts latex2text.Options
		in   string
		want string
	}{
		{"accents", latex2text.DefaultOptions(), `\'{e}t\'{e} \textbf{caf\"e} --- \emph{x} \ss{} \AA`, "été cafë — x ß Å"},
		{"math", delims, `Let $\alpha + \beta \leq \gamma$ hold.`, "Let $α + β≤γ$ hold."},
		{"structure", latex2text.DefaultOptions(), "\\section{Intro} Some text.\n\n\\begin{itemize}\\item one \\item two\\end{itemize}",
			"\n\n§ INTRO\n Some text.\n\n\n  * one \n  * two"},
		{"fill", fill, "The quick brown fox jumps over the lazy dog, then naps in the warm afternoon sun.\n\nA second paragraph.",
			"The quick brown fox\njumps over the lazy dog,\nthen naps in the warm\nafternoon sun.\n\nA second paragraph."},
		{"matrix", latex2text.DefaultOptions(), `$\begin{pmatrix} 1 & 22 \\ 333 & 4 \end{pmatrix}$`, "[   1  22; 333   4 ]"},
	}
	for _, c := range cases {
		if got := convert(t, c.opts, c.in); got != c.want {
			t.Errorf("%s: got %q; want %q", c.name, got, c.want)
		}
	}
}

func TestInvalidMathMode(t *testing.T) {
	opts := latex2text.DefaultOptions()
	opts.MathMode = "nope"
	_, err := latex2text.NewLatexNodes2Text(opts)
	var be *pylatexenc.BuiltinError
	if !errors.As(err, &be) || be.Class != "ValueError" ||
		be.Message != "math_mode= option must be one of 'text', 'with-delimiters', 'verbatim', 'remove'" {
		t.Errorf("err = %v", err)
	}
}

func TestCustomSpecs(t *testing.T) {
	wdb := latexwalker.GetDefaultLatexContextDb()
	parser, err := latexwalker.NewMacroStandardArgsParser("{{", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := wdb.AddContextCategory("custom", []*latexwalker.MacroSpec{{MacroName: "foo", ArgsParser: parser}},
		nil, nil, latexwalker.Placement{Prepend: true}); err != nil {
		t.Fatal(err)
	}
	tdb := latex2text.GetDefaultLatexContextDb()
	if err := tdb.AddContextCategory("custom", []*latex2text.MacroTextSpec{latex2text.NewMacroTextSpec("foo", "<%(2)s|%(1)s>")},
		nil, nil, latex2text.Placement{Prepend: true}); err != nil {
		t.Fatal(err)
	}
	opts := latex2text.DefaultOptions()
	opts.LatexContext = tdb
	wopts := latexwalker.DefaultOptions()
	wopts.LatexContext = wdb
	got, err := latex2text.LatexToText(`\foo{a}{b} and \foo{x}{\'e}`, opts, wopts)
	if err != nil {
		t.Fatal(err)
	}
	if want := "<b|a> and <é|x>"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
	if err := tdb.AddContextCategory("custom", nil, nil, nil, latex2text.Placement{}); err == nil ||
		err.Error() != "ValueError: Category custom is already registered in the context database" {
		t.Errorf("re-adding a category: err = %v", err)
	}
}
