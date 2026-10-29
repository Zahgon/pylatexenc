package latexwalker

// ParsingState is the state a node was parsed in.
type ParsingState struct {
	S                 string
	LatexContext      *LatexContextDb
	InMathMode        bool
	MathModeDelimiter string // "" when None

	runes []rune
}

// subContext is ParsingState.sub_context(): a copy with some fields replaced, after which a
// math-mode delimiter is dropped when the copy is not in math mode.
func (ps *ParsingState) subContext(setMath bool, inMath bool, setDelim bool, delim string) *ParsingState {
	p := *ps
	if setMath {
		p.InMathMode = inMath
	}
	if setDelim {
		p.MathModeDelimiter = delim
	}
	if !p.InMathMode && p.MathModeDelimiter != "" {
		p.MathModeDelimiter = ""
	}
	return &p
}

// Node is a node of the parsed tree: one of *CharsNode, *GroupNode, *CommentNode,
// *MacroNode, *EnvironmentNode, *SpecialsNode and *MathNode.
type Node interface {
	Base() *NodeBase
	LatexVerbatim() string
}

// NodeBase holds what every node has: the parsing state it was parsed in and its position
// and length in the input, counted in code points.
type NodeBase struct {
	ParsingState *ParsingState
	Pos          int
	Len          int
}

// Base returns the node's common fields.
func (n *NodeBase) Base() *NodeBase { return n }

// LatexVerbatim is the slice of the input the node was parsed from.
func (n *NodeBase) LatexVerbatim() string {
	if n.ParsingState == nil {
		return ""
	}
	r := n.ParsingState.runes
	if r == nil {
		r = []rune(n.ParsingState.S)
	}
	return string(sliceRunes(r, n.Pos, n.Pos+n.Len))
}

func sliceRunes(r []rune, a, b int) []rune {
	if a < 0 {
		a = 0
	}
	if b > len(r) {
		b = len(r)
	}
	if a >= b {
		return nil
	}
	return r[a:b]
}

// CharsNode is LatexCharsNode.
type CharsNode struct {
	NodeBase
	Chars string
}

// GroupNode is LatexGroupNode.
type GroupNode struct {
	NodeBase
	Nodelist   []Node
	Delimiters [2]string
}

// CommentNode is LatexCommentNode.
type CommentNode struct {
	NodeBase
	Comment          string
	CommentPostSpace string
}

// MacroNode is LatexMacroNode. NodeOptArg and NodeArgs are the legacy nodeoptarg and nodeargs
// attributes; NodeArgs is nil exactly when Python's nodeargs is None.
type MacroNode struct {
	NodeBase
	MacroName      string
	Nodeargd       *ParsedMacroArgs
	MacroPostSpace string
	NodeOptArg     Node
	NodeArgs       []Node
}

// EnvironmentNode is LatexEnvironmentNode. OptArgs and Args are the legacy optargs and args
// attributes.
type EnvironmentNode struct {
	NodeBase
	EnvironmentName string
	Nodelist        []Node
	Nodeargd        *ParsedMacroArgs
	OptArgs         []Node
	Args            []Node
}

// SpecialsNode is LatexSpecialsNode.
type SpecialsNode struct {
	NodeBase
	SpecialsChars string
	Nodeargd      *ParsedMacroArgs
}

// MathNode is LatexMathNode.
type MathNode struct {
	NodeBase
	Displaytype string
	Nodelist    []Node
	Delimiters  [2]string
}

// ParsedMacroArgs holds the parsed arguments of a macro, environment or specials. Argnlist
// has one entry per Argspec character; an absent optional argument is nil. Verbatim is set
// when a VerbatimArgsParser parsed the arguments.
type ParsedMacroArgs struct {
	Argspec  string
	Argnlist []Node
	Verbatim *VerbatimArgs
}

// VerbatimArgs is what ParsedVerbatimArgs adds: the verbatim text and its delimiters, which
// are both "" when Python's verbatim_delimiters is None.
type VerbatimArgs struct {
	Text       string
	Delimiters [2]string
}

// legacy is ParsedMacroArgs._get_legacy_attribs: (nodeoptarg, nodeargs).
func (a *ParsedMacroArgs) legacy() (Node, []Node) {
	if a == nil {
		return nil, []Node{}
	}
	spec := []rune(a.Argspec)
	nskip := 0
	for len(spec) > 0 && spec[0] == '*' {
		spec = spec[1:]
		nskip++
	}
	allBraces := true
	for _, c := range spec[min(1, len(spec)):] {
		if c != '{' {
			allBraces = false
		}
	}
	if len(spec) > 0 && spec[0] == '[' && allBraces && nskip < len(a.Argnlist) {
		rest := append([]Node{}, a.Argnlist[nskip+1:]...)
		return a.Argnlist[nskip], rest
	}
	args := a.Argnlist
	if args == nil {
		args = []Node{}
	}
	return nil, args
}
