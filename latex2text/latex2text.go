// Package latex2text converts parsed LaTeX to plain text, as pylatexenc.latex2text 2.11
// does.
package latex2text

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/phfaist/pylatexenc"
	"github.com/phfaist/pylatexenc/internal/pyrt"
	"github.com/phfaist/pylatexenc/latexwalker"
)

// Options are the LatexNodes2Text constructor options.
//
// StrictLatexSpaces takes the values Python's strict_latex_spaces does: nil (None), a bool,
// a preset name ("based-on-source", "macros", "except-in-equations", "on", "off"), or a
// map[string]any of the keys "between-macro-and-chars", "between-latex-constructs",
// "after-comment" (bools) and "in-equations" (nil or any value StrictLatexSpaces takes).
// FillText 0 turns filling off; a nil LatexContext means the default text specs.
type Options struct {
	LatexContext           *LatexContextDb
	MathMode               string
	KeepComments           bool
	StrictLatexSpaces      any
	KeepBracedGroups       bool
	KeepBracedGroupsMinlen int
	FillText               int
}

// DefaultOptions are the Python defaults.
func DefaultOptions() Options {
	return Options{MathMode: "text", StrictLatexSpaces: false, KeepBracedGroupsMinlen: 2}
}

type spacesDict struct {
	betweenMacroAndChars   bool
	betweenLatexConstructs bool
	afterComment           bool
	inEquations            any
}

var spacesPresets = map[string]spacesDict{
	"based-on-source":     {},
	"macros":              {true, true, false, "based-on-source"},
	"except-in-equations": {true, true, true, "based-on-source"},
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case int:
		return x != 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

func parseStrictSpaces(v any) (spacesDict, error) {
	switch x := v.(type) {
	case nil:
		return spacesDict{}, nil
	case bool:
		if !x {
			return spacesPresets["macros"], nil
		}
		return spacesDict{true, true, true, true}, nil
	case map[string]any:
		d := spacesDict{}
		for k, val := range x {
			switch k {
			case "between-macro-and-chars":
				d.betweenMacroAndChars = truthy(val)
			case "between-latex-constructs":
				d.betweenLatexConstructs = truthy(val)
			case "after-comment":
				d.afterComment = truthy(val)
			case "in-equations":
				d.inEquations = val
			}
		}
		return d, nil
	case string:
		switch x {
		case "on":
			return parseStrictSpaces(true)
		case "off":
			return parseStrictSpaces(false)
		}
		if d, ok := spacesPresets[x]; ok {
			return d, nil
		}
		return spacesDict{}, valueError(fmt.Sprintf("invalid value for strict_latex_spaces preset: %s", x))
	}
	return spacesDict{}, valueError(fmt.Sprintf("Invalid value for strict_latex_spaces: %v", v))
}

// LatexNodes2Text converts nodes to text. It remembers the \title, \author and \date it
// converts, for later \maketitle.
type LatexNodes2Text struct {
	LatexContext           *LatexContextDb
	MathMode               string
	KeepComments           bool
	KeepBracedGroups       bool
	KeepBracedGroupsMinlen int
	FillText               int

	strict    spacesDict
	docTitle  *string
	docAuthor *string
	docDate   *string
}

// NewLatexNodes2Text checks the options as the Python constructor does.
func NewLatexNodes2Text(opts Options) (*LatexNodes2Text, error) {
	ctx := opts.LatexContext
	if ctx == nil {
		ctx = GetDefaultLatexContextDb()
	}
	switch opts.MathMode {
	case "text", "with-delimiters", "verbatim", "remove":
	default:
		return nil, valueError("math_mode= option must be one of 'text', 'with-delimiters', 'verbatim', 'remove'")
	}
	strict, err := parseStrictSpaces(opts.StrictLatexSpaces)
	if err != nil {
		return nil, err
	}
	return &LatexNodes2Text{LatexContext: ctx, MathMode: opts.MathMode, KeepComments: opts.KeepComments,
		KeepBracedGroups: opts.KeepBracedGroups, KeepBracedGroupsMinlen: opts.KeepBracedGroupsMinlen,
		FillText: opts.FillText, strict: strict}, nil
}

func valueError(msg string) error { return &pylatexenc.BuiltinError{Class: "ValueError", Message: msg} }

func typeError(msg string) error { return &pylatexenc.BuiltinError{Class: "TypeError", Message: msg} }

func indexError(msg string) error { return &pylatexenc.BuiltinError{Class: "IndexError", Message: msg} }

func attributeError(msg string) error {
	return &pylatexenc.BuiltinError{Class: "AttributeError", Message: msg}
}

// LatexToText parses latex with a walker built from walkerOpts and converts the nodes.
func (l *LatexNodes2Text) LatexToText(latex string, walkerOpts latexwalker.Options) (string, error) {
	nodes, _, _, err := latexwalker.NewLatexWalker(latex, walkerOpts).GetLatexNodes(0, latexwalker.NodesOptions{})
	if err != nil {
		return "", err
	}
	return l.NodelistToText(nodes)
}

// LatexToText is LatexNodes2Text(**opts).latex_to_text(latex, **walker options).
func LatexToText(latex string, opts Options, walkerOpts latexwalker.Options) (string, error) {
	l, err := NewLatexNodes2Text(opts)
	if err != nil {
		return "", err
	}
	return l.LatexToText(latex, walkerOpts)
}

func (l *LatexNodes2Text) isBareMacroNode(n latexwalker.Node) (bool, error) {
	m, ok := n.(*latexwalker.MacroNode)
	if !ok || m == nil || m.NodeOptArg != nil {
		return false, nil
	}
	if m.NodeArgs == nil {
		return false, typeError("object of type 'NoneType' has no len()")
	}
	return len(m.NodeArgs) == 0, nil
}

// NodelistToText converts a list of nodes.
func (l *LatexNodes2Text) NodelistToText(nodelist []latexwalker.Node) (string, error) {
	var b strings.Builder
	textcol := 0
	add := func(t string) {
		b.WriteString(t)
		if k := strings.LastIndexByte(t, '\n'); k >= 0 {
			textcol = pyrt.Len(t[k+1:])
		} else {
			textcol += pyrt.Len(t)
		}
	}
	var prev latexwalker.Node
	for _, node := range nodelist {
		bare, err := l.isBareMacroNode(prev)
		if err != nil {
			return "", err
		}
		if bare {
			if node == nil {
				return "", attributeError("'NoneType' object has no attribute 'isNodeType'")
			}
			if _, ok := node.(*latexwalker.CharsNode); ok && !l.strict.betweenMacroAndChars {
				add(prev.(*latexwalker.MacroNode).MacroPostSpace)
			}
		}
		t, err := l.NodeToText(node, textcol)
		if err != nil {
			return "", err
		}
		add(t)
		prev = node
	}
	return b.String(), nil
}

// NodeToText converts one node; textcol is the column the text starts at, which filling
// uses.
func (l *LatexNodes2Text) NodeToText(node latexwalker.Node, textcol int) (string, error) {
	switch n := node.(type) {
	case nil:
		return "", nil
	case *latexwalker.CharsNode:
		return l.charsNodeToText(n, textcol)
	case *latexwalker.CommentNode:
		return l.commentNodeToText(n), nil
	case *latexwalker.GroupNode:
		return l.groupNodeToText(n)
	case *latexwalker.MacroNode:
		return l.macroNodeToText(n)
	case *latexwalker.EnvironmentNode:
		return l.environmentNodeToText(n)
	case *latexwalker.SpecialsNode:
		return l.specialsNodeToText(n)
	case *latexwalker.MathNode:
		return l.MathNodeToText(n)
	}
	return "", nil
}

func (l *LatexNodes2Text) charsNodeToText(n *latexwalker.CharsNode, textcol int) (string, error) {
	content := n.Chars
	if l.FillText != 0 {
		var err error
		if content, err = l.doFillText(content, textcol); err != nil {
			return "", err
		}
	}
	if !l.strict.betweenLatexConstructs && pyrt.Strip(content) == "" {
		return "", nil
	}
	return content, nil
}

func (l *LatexNodes2Text) commentNodeToText(n *latexwalker.CommentNode) string {
	if l.KeepComments {
		if l.strict.afterComment {
			nl := "\n"
			if n.CommentPostSpace == "" {
				nl = ""
			}
			return "%" + n.Comment + nl
		}
		return "%" + n.Comment + n.CommentPostSpace
	}
	if l.strict.afterComment {
		return ""
	}
	return n.CommentPostSpace
}

func (l *LatexNodes2Text) groupNodeToText(n *latexwalker.GroupNode) (string, error) {
	contents, err := l.NodelistToText(n.Nodelist)
	if err != nil {
		return "", err
	}
	if l.KeepBracedGroups && pyrt.Len(contents) >= l.KeepBracedGroupsMinlen {
		return n.Delimiters[0] + contents + n.Delimiters[1], nil
	}
	return contents, nil
}

func (l *LatexNodes2Text) groupNodeContentsToText(n latexwalker.Node) (string, error) {
	switch g := n.(type) {
	case nil:
		return "", nil
	case *latexwalker.GroupNode:
		return l.NodelistToText(g.Nodelist)
	}
	return l.NodeToText(n, 0)
}

func nodeargd(n latexwalker.Node) *latexwalker.ParsedMacroArgs {
	switch x := n.(type) {
	case *latexwalker.MacroNode:
		return x.Nodeargd
	case *latexwalker.EnvironmentNode:
		return x.Nodeargd
	case *latexwalker.SpecialsNode:
		return x.Nodeargd
	}
	return nil
}

func argnlist(n latexwalker.Node) []latexwalker.Node {
	if a := nodeargd(n); a != nil && len(a.Argnlist) > 0 {
		return a.Argnlist
	}
	return nil
}

func (l *LatexNodes2Text) argsText(n latexwalker.Node) (string, error) {
	var b strings.Builder
	for _, a := range argnlist(n) {
		t, err := l.groupNodeContentsToText(a)
		if err != nil {
			return "", err
		}
		b.WriteString(t)
	}
	return b.String(), nil
}

func (l *LatexNodes2Text) macroNodeToText(n *latexwalker.MacroNode) (string, error) {
	mac := l.LatexContext.GetMacroSpec(n.MacroName)
	if mac == nil {
		mac = &MacroTextSpec{Discard: true}
	}
	if mac.SimplifyFunc != nil || mac.SimplifyRepl != "" {
		return l.applySimplifyRepl(n, mac.SimplifyRepl, mac.SimplifyFunc)
	}
	if mac.Discard {
		return "", nil
	}
	return l.argsText(n)
}

func (l *LatexNodes2Text) environmentNodeToText(n *latexwalker.EnvironmentNode) (string, error) {
	env := l.LatexContext.GetEnvironmentSpec(n.EnvironmentName)
	if env == nil {
		env = &EnvironmentTextSpec{}
	}
	if env.SimplifyFunc != nil || env.SimplifyRepl != "" {
		return l.applySimplifyRepl(n, env.SimplifyRepl, env.SimplifyFunc)
	}
	if env.Discard {
		return "", nil
	}
	return l.NodelistToText(n.Nodelist)
}

func (l *LatexNodes2Text) specialsNodeToText(n *latexwalker.SpecialsNode) (string, error) {
	spec := l.LatexContext.GetSpecialsSpec(n.SpecialsChars)
	if spec == nil {
		return n.SpecialsChars, nil
	}
	if spec.SimplifyFunc != nil || spec.SimplifyRepl != "" {
		return l.applySimplifyRepl(n, spec.SimplifyRepl, spec.SimplifyFunc)
	}
	return "", attributeError("'SpecialsTextSpec' object has no attribute 'discard'")
}

// withEquationContext runs fn with the strict_latex_spaces settings of equations.
func (l *LatexNodes2Text) withEquationContext(fn func() (string, error)) (string, error) {
	if l.strict.inEquations == nil {
		return fn()
	}
	inner, err := parseStrictSpaces(l.strict.inEquations)
	if err != nil {
		return "", err
	}
	saved := l.strict
	l.strict = inner
	defer func() { l.strict = saved }()
	return fn()
}

// MathNodeToText converts a math node, or a math environment node, according to MathMode.
func (l *LatexNodes2Text) MathNodeToText(node latexwalker.Node) (string, error) {
	var nodelist []latexwalker.Node
	var delims [2]string
	display := true
	switch n := node.(type) {
	case *latexwalker.MathNode:
		nodelist, delims = n.Nodelist, n.Delimiters
		display = n.Displaytype == "display"
	case *latexwalker.EnvironmentNode:
		nodelist = n.Nodelist
		delims = [2]string{`\begin{` + n.EnvironmentName + `}`, `\end{` + n.EnvironmentName + `}`}
	default:
		return "", attributeError("node has no math content")
	}
	switch l.MathMode {
	case "verbatim":
		if display {
			return l.fmtIndentedBlock(node.LatexVerbatim(), ""), nil
		}
		return node.LatexVerbatim(), nil
	case "remove":
		return "", nil
	}
	content, err := l.withEquationContext(func() (string, error) { return l.NodelistToText(nodelist) })
	if err != nil {
		return "", err
	}
	content = pyrt.Strip(content)
	if l.MathMode == "with-delimiters" {
		if display {
			return delims[0] + l.fmtIndentedBlock(content, "") + delims[1], nil
		}
		return delims[0] + content + delims[1], nil
	}
	if display {
		return l.fmtIndentedBlock(content, "    "), nil
	}
	return content, nil
}

func (l *LatexNodes2Text) fmtIndentedBlock(contents, indent string) string {
	block := "\n" + indent + strings.ReplaceAll(contents, "\n", "\n"+indent) + "\n"
	if l.FillText != 0 {
		block = "\n" + block + "\n"
	}
	return block
}

func leadingSpace(s string) string { return s[:len(s)-len(pyrt.LStrip(s))] }

func trailingSpace(s string) string { return s[len(pyrt.RStrip(s)):] }

// splitParagraphs is re.split(r'\n{2,}', text).
func splitParagraphs(text string) []string {
	var out []string
	start := 0
	for i := 0; i < len(text); {
		if text[i] != '\n' {
			i++
			continue
		}
		j := i
		for j < len(text) && text[j] == '\n' {
			j++
		}
		if j-i >= 2 {
			out = append(out, text[start:i])
			start = j
		}
		i = j
	}
	return append(out, text[start:])
}

func (l *LatexNodes2Text) doFillText(text string, textcol int) (string, error) {
	headWs := leadingSpace(text)
	headPar := ""
	if strings.Contains(headWs, "\n\n") {
		headPar = "\n\n"
	}
	trailWs := trailingSpace(text)
	trailPar := ""
	if strings.Contains(trailWs, "\n\n") {
		trailPar = "\n\n"
	}
	text = pyrt.Strip(text)
	fillChunk := func(x string, col int) (string, error) {
		x = pyrt.Strip(x)
		if col >= l.FillText-4 {
			f, err := pyrt.Fill(x, l.FillText, "")
			return "\n" + f, err
		}
		f, err := pyrt.Fill(x, l.FillText, strings.Repeat("X", col))
		if err != nil {
			return "", err
		}
		r := []rune(f)
		if col > len(r) {
			col = len(r)
		}
		return string(r[col:]), nil
	}
	var chunks []string
	for j, x := range splitParagraphs(text) {
		col := 0
		if j == 0 {
			col = textcol
		}
		c, err := fillChunk(x, col)
		if err != nil {
			return "", err
		}
		if pyrt.Strip(c) != "" {
			chunks = append(chunks, c)
		}
	}
	var b strings.Builder
	b.WriteString(headPar)
	if textcol > 0 && headWs != "" && headPar == "" {
		b.WriteByte(' ')
	}
	b.WriteString(strings.Join(chunks, "\n\n"))
	if trailWs != "" && trailPar == "" {
		b.WriteByte(' ')
	}
	b.WriteString(trailPar)
	return b.String(), nil
}

func (l *LatexNodes2Text) applySimplifyRepl(node latexwalker.Node, repl string, fn ReplFunc) (string, error) {
	if fn != nil {
		return fn(node, l)
	}
	if !strings.Contains(repl, "%") || pyrt.Len(repl) == 1 {
		return repl, nil
	}
	nodeargs := argnlist(node)
	hasS := pyrt.HasPercentS(repl)
	argTexts := func() ([]string, error) {
		texts := []string{}
		for _, a := range nodeargs {
			t, err := l.groupNodeContentsToText(a)
			if err != nil {
				return nil, err
			}
			texts = append(texts, t)
		}
		return texts, nil
	}
	asDict := func(texts []string) *pyrt.StrDict {
		d := pyrt.NewStrDict()
		for j, t := range texts {
			d.Set(strconv.Itoa(j+1), t)
		}
		return d
	}
	var args pyrt.PercentArgs
	if env, ok := node.(*latexwalker.EnvironmentNode); ok {
		if hasS {
			body, err := l.NodelistToText(env.Nodelist)
			if err != nil {
				return "", err
			}
			args.Tuple = []string{body}
		} else {
			texts, err := argTexts()
			if err != nil {
				return "", err
			}
			body, err := l.NodelistToText(env.Nodelist)
			if err != nil {
				return "", err
			}
			args.Dict = asDict(texts)
			args.Dict.Set("body", body)
		}
	} else {
		texts, err := argTexts()
		if err != nil {
			return "", err
		}
		if hasS {
			args.Tuple = texts
		} else {
			args.Dict = asDict(texts)
		}
	}
	out, err := pyrt.Percent(repl, args)
	if err != nil {
		if be, ok := err.(*pylatexenc.BuiltinError); ok && (be.Class == "TypeError" || be.Class == "ValueError") {
			return repl, nil
		}
		return "", err
	}
	return out, nil
}

// NodeArgToText is node_arg_to_text(): the text of the k-th parsed argument of a node, or ""
// when the node has no parsed arguments.
func (l *LatexNodes2Text) NodeArgToText(node latexwalker.Node, k int) (string, error) {
	args := argnlist(node)
	if args == nil {
		return "", nil
	}
	if k < 0 || k >= len(args) {
		return "", indexError("list index out of range")
	}
	return l.groupNodeContentsToText(args[k])
}
