package latexwalker

import (
	"strings"

	"github.com/phfaist/pylatexenc"
)

func valueError(msg string) error { return &pylatexenc.BuiltinError{Class: "ValueError", Message: msg} }

func typeError(msg string) error { return &pylatexenc.BuiltinError{Class: "TypeError", Message: msg} }

// NodelistToLatex writes nodes back as LaTeX, as nodelist_to_latex() does: only what the
// nodes record is written, so the spacing between a macro's arguments and the comments
// among them are not.
func NodelistToLatex(nodelist []Node) string {
	var b strings.Builder
	writeNodes(&b, nodelist)
	return b.String()
}

func writeArgs(b *strings.Builder, a *ParsedMacroArgs) {
	if a == nil {
		return
	}
	spec := []rune(a.Argspec)
	for i := 0; i < len(spec) && i < len(a.Argnlist); i++ {
		if a.Argnlist[i] != nil {
			writeNodes(b, []Node{a.Argnlist[i]})
		}
	}
}

func writeNodes(b *strings.Builder, nodelist []Node) {
	for _, n := range nodelist {
		switch n := n.(type) {
		case nil:
		case *CharsNode:
			b.WriteString(n.Chars)
		case *MacroNode:
			b.WriteString(`\` + n.MacroName + n.MacroPostSpace)
			writeArgs(b, n.Nodeargd)
		case *SpecialsNode:
			b.WriteString(n.SpecialsChars)
			writeArgs(b, n.Nodeargd)
		case *CommentNode:
			b.WriteString("%" + n.Comment + n.CommentPostSpace)
		case *GroupNode:
			b.WriteString(n.Delimiters[0])
			writeNodes(b, n.Nodelist)
			b.WriteString(n.Delimiters[1])
		case *EnvironmentNode:
			b.WriteString(`\begin{` + n.EnvironmentName + `}`)
			writeArgs(b, n.Nodeargd)
			writeNodes(b, n.Nodelist)
			b.WriteString(`\end{` + n.EnvironmentName + `}`)
		case *MathNode:
			b.WriteString(n.Delimiters[0])
			writeNodes(b, n.Nodelist)
			b.WriteString(n.Delimiters[1])
		}
	}
}
