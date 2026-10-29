package latexwalker_test

import (
	"errors"
	"testing"

	"github.com/phfaist/pylatexenc/latexwalker"
)

func kind(n latexwalker.Node) (string, string) {
	switch x := n.(type) {
	case *latexwalker.CharsNode:
		return "chars", ""
	case *latexwalker.MacroNode:
		return "macro", x.MacroName
	case *latexwalker.MathNode:
		return "math", x.Displaytype
	case *latexwalker.CommentNode:
		return "comment", x.Comment
	case *latexwalker.EnvironmentNode:
		return "environment", x.EnvironmentName
	case *latexwalker.GroupNode:
		return "group", ""
	case *latexwalker.SpecialsNode:
		return "specials", x.SpecialsChars
	}
	return "?", ""
}

func TestGetLatexNodes(t *testing.T) {
	s := "Hello \\textbf{world} $x^2$ % c\n\\begin{itemize}\\item A\\end{itemize}"
	w := latexwalker.NewLatexWalker(s, latexwalker.DefaultOptions())
	nodes, pos, n, err := w.GetLatexNodes(0, latexwalker.NodesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if pos != 0 || n != 66 {
		t.Fatalf("pos, len = %d, %d; want 0, 66", pos, n)
	}
	want := []struct {
		kind, name string
		pos, len   int
	}{
		{"chars", "", 0, 6}, {"macro", "textbf", 6, 14}, {"chars", "", 20, 1}, {"math", "inline", 21, 5},
		{"chars", "", 26, 1}, {"comment", " c", 27, 4}, {"environment", "itemize", 31, 35},
	}
	if len(nodes) != len(want) {
		t.Fatalf("got %d nodes, want %d", len(nodes), len(want))
	}
	for i, node := range nodes {
		k, name := kind(node)
		b := node.Base()
		if k != want[i].kind || name != want[i].name || b.Pos != want[i].pos || b.Len != want[i].len {
			t.Errorf("node %d = %s %q @%d+%d; want %s %q @%d+%d", i, k, name, b.Pos, b.Len,
				want[i].kind, want[i].name, want[i].pos, want[i].len)
		}
	}
	if got := latexwalker.NodelistToLatex(nodes); got != s {
		t.Errorf("NodelistToLatex = %q; want the input back", got)
	}
}

func TestStrictParseError(t *testing.T) {
	w := latexwalker.NewLatexWalker("\\textbf{x\n\\emph{y}", latexwalker.Options{})
	_, _, _, err := w.GetLatexNodes(0, latexwalker.NodesOptions{})
	var pe *latexwalker.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v; want a *ParseError", err)
	}
	want := "Unexpected end of stream, was expecting '}' @(2,8)\nOpen LaTeX blocks:\n" +
		"            @(1,0)  arguments of macro \"textbf\"\n"
	if pe.Error() != want {
		t.Errorf("message = %q; want %q", pe.Error(), want)
	}
}

func TestTokens(t *testing.T) {
	w := latexwalker.NewLatexWalker("  \\alpha  x", latexwalker.DefaultOptions())
	tok, err := w.GetToken(0, nil, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Tok != "macro" || tok.Arg != "alpha" || tok.Pos != 2 || tok.Len != 8 || tok.PreSpace != "  " || tok.PostSpace != "  " {
		t.Errorf("token = %+v", *tok)
	}
	_, err = latexwalker.NewLatexWalker("  ", latexwalker.DefaultOptions()).GetToken(0, nil, true, nil)
	var eos *latexwalker.EndOfStreamError
	if !errors.As(err, &eos) || eos.FinalSpace != "  " {
		t.Errorf("err = %v; want end of stream after two spaces", err)
	}
}

func TestPosToLinenoColno(t *testing.T) {
	line, col := latexwalker.NewLatexWalker("ab\ncd\n\nef", latexwalker.DefaultOptions()).PosToLinenoColno(7)
	if line != 4 || col != 0 {
		t.Errorf("PosToLinenoColno(7) = %d, %d; want 4, 0", line, col)
	}
}
