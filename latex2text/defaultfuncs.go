package latex2text

import (
	"fmt"
	"strings"
	"time"

	"github.com/phfaist/pylatexenc/internal/pyrt"
	"github.com/phfaist/pylatexenc/latexwalker"
)

var mathStyleOffsets = map[string][2]rune{
	"bold":             {0x1D400, 0x1D41A},
	"italic":           {0x1D434, 0x1D44E},
	"bold-italic":      {0x1D468, 0x1D482},
	"script":           {0x1D49C, 0x1D4B6},
	"bold-script":      {0x1D4D0, 0x1D4EA},
	"fraktur":          {0x1D504, 0x1D51E},
	"doublestruck":     {0x1D538, 0x1D552},
	"bold-fraktur":     {0x1D56C, 0x1D586},
	"sans":             {0x1D5A0, 0x1D5BA},
	"sans-bold":        {0x1D5D4, 0x1D5EE},
	"sans-italic":      {0x1D608, 0x1D622},
	"sans-bold-italic": {0x1D63C, 0x1D656},
	"monospace":        {0x1D670, 0x1D68A},
}

var mathStyleExceptions = map[string]map[rune]rune{
	"italic": {'h': 0x210E},
	"script": {'B': 0x212C, 'E': 0x2130, 'F': 0x2131, 'H': 0x210B, 'I': 0x2110, 'L': 0x2112,
		'M': 0x2133, 'R': 0x211B, 'e': 0x212F, 'g': 0x210A, 'o': 0x2134},
	"fraktur":      {'C': 0x212D, 'H': 0x210C, 'I': 0x2111, 'R': 0x211C, 'Z': 0x2128},
	"doublestruck": {'C': 0x2102, 'H': 0x210D, 'N': 0x2115, 'P': 0x2119, 'Q': 0x211A, 'R': 0x211D, 'Z': 0x2124},
}

// FmtMathTextStyle is fmt_math_text_style(): the ASCII letters of text in a Unicode
// mathematical alphanumeric style.
func FmtMathTextStyle(text, style string) string {
	off, ok := mathStyleOffsets[style]
	if !ok {
		off = [2]rune{'A', 'a'}
	}
	var b strings.Builder
	for _, c := range text {
		if z, ok := mathStyleExceptions[style][c]; ok {
			b.WriteRune(z)
		} else if c >= 'A' && c <= 'Z' {
			b.WriteRune(off[0] + c - 'A')
		} else if c >= 'a' && c <= 'z' {
			b.WriteRune(off[1] + c - 'a')
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func latexToday() string {
	now := time.Now()
	return fmt.Sprintf("%s %d, %d", now.Month().String(), now.Day(), now.Year())
}

func pyTypeName(n latexwalker.Node) string {
	switch n.(type) {
	case *latexwalker.CharsNode:
		return "LatexCharsNode"
	case *latexwalker.GroupNode:
		return "LatexGroupNode"
	case *latexwalker.CommentNode:
		return "LatexCommentNode"
	case *latexwalker.EnvironmentNode:
		return "LatexEnvironmentNode"
	case *latexwalker.SpecialsNode:
		return "LatexSpecialsNode"
	case *latexwalker.MathNode:
		return "LatexMathNode"
	}
	return "LatexMacroNode"
}

// macroOf returns the node as a macro node, or the AttributeError Python raises when it
// reads a macro-only attribute from another node.
func macroOf(n latexwalker.Node, attr string) (*latexwalker.MacroNode, error) {
	if m, ok := n.(*latexwalker.MacroNode); ok {
		return m, nil
	}
	return nil, attributeError(fmt.Sprintf("'%s' object has no attribute '%s'", pyTypeName(n), attr))
}

func mathStyleRepl(style string) ReplFunc {
	return func(n latexwalker.Node, l *LatexNodes2Text) (string, error) {
		t, err := l.NodeArgToText(n, 0)
		if err != nil {
			return "", err
		}
		return FmtMathTextStyle(t, style), nil
	}
}

func inputMacroRepl(n latexwalker.Node, l *LatexNodes2Text) (string, error) {
	m, err := macroOf(n, "nodeargs")
	if err != nil {
		return "", err
	}
	if m.NodeArgs == nil {
		return "", typeError("object of type 'NoneType' has no len()")
	}
	if len(m.NodeArgs) == 0 {
		return "", indexError("list index out of range")
	}
	if _, err := l.NodelistToText([]latexwalker.Node{m.NodeArgs[0]}); err != nil {
		return "", err
	}
	return "", nil
}

func texorpdfstringRepl(n latexwalker.Node, l *LatexNodes2Text) (string, error) {
	m, err := macroOf(n, "nodeargs")
	if err != nil {
		return "", err
	}
	if m.NodeArgs == nil {
		return "", typeError("'NoneType' object is not subscriptable")
	}
	return l.NodelistToText(m.NodeArgs[min(1, len(m.NodeArgs)):min(2, len(m.NodeArgs))])
}

func percentRepl(n latexwalker.Node, l *LatexNodes2Text) (string, error) { return "%", nil }

func docFieldRepl(field func(l *LatexNodes2Text) **string) ReplFunc {
	return func(n latexwalker.Node, l *LatexNodes2Text) (string, error) {
		a := nodeargd(n)
		if a == nil {
			return "", attributeError("'NoneType' object has no attribute 'argnlist'")
		}
		t, err := l.NodelistToText(a.Argnlist[:min(1, len(a.Argnlist))])
		if err != nil {
			return "", err
		}
		*field(l) = &t
		return "", nil
	}
}

var (
	titleRepl  = docFieldRepl(func(l *LatexNodes2Text) **string { return &l.docTitle })
	authorRepl = docFieldRepl(func(l *LatexNodes2Text) **string { return &l.docAuthor })
	dateRepl   = docFieldRepl(func(l *LatexNodes2Text) **string { return &l.docDate })
)

func maketitleRepl(n latexwalker.Node, l *LatexNodes2Text) (string, error) {
	or := func(p *string, def string) string {
		if p != nil {
			return *p
		}
		return def
	}
	title := or(l.docTitle, `[NO \title GIVEN]`)
	author := or(l.docAuthor, `[NO \author GIVEN]`)
	date := or(l.docDate, latexToday())
	width := max(pyrt.Len(title), 4+pyrt.Len(author), 4+pyrt.Len(date))
	return title + "\n" + "    " + author + "\n" + "    " + date + "\n" + strings.Repeat("=", width) + "\n\n", nil
}

func itemRepl(n latexwalker.Node, l *LatexNodes2Text) (string, error) {
	m, err := macroOf(n, "nodeoptarg")
	if err != nil {
		return "", err
	}
	if m.NodeOptArg == nil {
		return "\n  * ", nil
	}
	t, err := l.NodelistToText([]latexwalker.Node{m.NodeOptArg})
	if err != nil {
		return "", err
	}
	return "\n  " + t, nil
}

func hrefRepl(n latexwalker.Node, l *LatexNodes2Text) (string, error) {
	a := nodeargd(n)
	if a == nil {
		return "", attributeError("'NoneType' object has no attribute 'argnlist'")
	}
	if len(a.Argnlist) < 2 {
		return "", indexError("list index out of range")
	}
	text, err := l.NodelistToText([]latexwalker.Node{a.Argnlist[1]})
	if err != nil {
		return "", err
	}
	url, err := l.NodelistToText([]latexwalker.Node{a.Argnlist[0]})
	if err != nil {
		return "", err
	}
	return text + " <" + url + ">", nil
}

func sectionRepl(prefix, suffix string, upper bool) ReplFunc {
	return func(n latexwalker.Node, l *LatexNodes2Text) (string, error) {
		t, err := l.NodeArgToText(n, 2)
		if err != nil {
			return "", err
		}
		if upper {
			t = pyrt.Upper(t)
		}
		return prefix + t + suffix, nil
	}
}

func uebungRepl(n latexwalker.Node, l *LatexNodes2Text) (string, error) {
	m, err := macroOf(n, "nodeargs")
	if err != nil {
		return "", err
	}
	if m.NodeArgs == nil {
		return "", typeError("'NoneType' object is not subscriptable")
	}
	if len(m.NodeArgs) == 0 {
		return "", indexError("list index out of range")
	}
	t, err := l.NodelistToText([]latexwalker.Node{m.NodeArgs[0]})
	if err != nil {
		return "", err
	}
	s := "\n" + t + "\n"
	if len(m.NodeArgs) < 2 {
		return "", indexError("list index out of range")
	}
	if opt := m.NodeArgs[1]; opt != nil {
		t, err := l.NodelistToText([]latexwalker.Node{opt})
		if err != nil {
			return "", err
		}
		s += "[" + t + "]\n"
	}
	return s, nil
}

func placeholderRepl(text string, block bool) ReplFunc {
	return func(n latexwalker.Node, l *LatexNodes2Text) (string, error) {
		chars := make([]string, 0, len(text))
		for _, c := range text {
			chars = append(chars, string(c))
		}
		txt := "< " + strings.Join(chars, " ") + " >"
		if block {
			return l.fmtIndentedBlock(txt, "    "), nil
		}
		return " " + txt + " ", nil
	}
}

func equationRepl(n latexwalker.Node, l *LatexNodes2Text) (string, error) { return l.MathNodeToText(n) }

func matrixRepl(n latexwalker.Node, l *LatexNodes2Text) (string, error) {
	env, ok := n.(*latexwalker.EnvironmentNode)
	if !ok {
		return "", attributeError(fmt.Sprintf("'%s' object has no attribute 'nodelist'", pyTypeName(n)))
	}
	var rows [][]string
	var column []string
	var buffer []latexwalker.Node
	newColumn := func() error {
		if len(buffer) > 0 {
			t, err := l.NodelistToText(buffer)
			if err != nil {
				return err
			}
			column = append(column, pyrt.Strip(t))
		}
		buffer = nil
		return nil
	}
	newRow := func() error {
		if err := newColumn(); err != nil {
			return err
		}
		rows = append(rows, column)
		column = nil
		return nil
	}
	for _, c := range env.Nodelist {
		if s, ok := c.(*latexwalker.SpecialsNode); ok && s.SpecialsChars == "&" {
			if err := newColumn(); err != nil {
				return "", err
			}
			continue
		}
		if m, ok := c.(*latexwalker.MacroNode); ok && m.MacroName == `\` {
			if err := newRow(); err != nil {
				return "", err
			}
			continue
		}
		buffer = append(buffer, c)
	}
	if err := newRow(); err != nil {
		return "", err
	}
	width, cellSeen := 0, false
	for _, row := range rows {
		for _, x := range row {
			width = max(width, pyrt.Len(x))
			cellSeen = true
		}
	}
	if !cellSeen {
		return "", valueError("max() iterable argument is empty")
	}
	parts := make([]string, len(rows))
	for i, row := range rows {
		cells := make([]string, len(row))
		for j, x := range row {
			cells[j] = pyrt.RJust(x, width)
		}
		parts[i] = strings.Join(cells, " ")
	}
	return "[ " + strings.Join(parts, "; ") + " ]", nil
}

func accentRepl(combining string) ReplFunc {
	return func(n latexwalker.Node, l *LatexNodes2Text) (string, error) {
		m, err := macroOf(n, "nodeargs")
		if err != nil {
			return "", err
		}
		c := " "
		if len(m.NodeArgs) > 0 {
			t, err := l.NodelistToText([]latexwalker.Node{m.NodeArgs[0]})
			if err != nil {
				return "", err
			}
			c = pyrt.Strip(t)
		}
		var b strings.Builder
		for _, ch := range c {
			switch ch {
			case 'ı':
				ch = 'i'
			case 'ȷ':
				ch = 'j'
			}
			b.WriteString(pyrt.NFC(string(ch) + combining))
		}
		return b.String(), nil
	}
}
