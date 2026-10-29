// Package latexwalker parses LaTeX into a tree of nodes, as pylatexenc.latexwalker 2.11 does,
// together with the spec types of pylatexenc.macrospec that drive it.
//
// Every position and length is counted in code points of the input.
package latexwalker

import (
	"fmt"
	"sort"
	"strings"

	"github.com/phfaist/pylatexenc/internal/pyrt"
)

// Token is LatexToken. For "specials" tokens Specials is the matched spec and Arg its
// characters.
type Token struct {
	Tok       string
	Arg       string
	Specials  *SpecialsSpec
	Pos       int
	Len       int
	PreSpace  string
	PostSpace string
}

// argStr is str() of the token's arg.
func (t *Token) argStr() string {
	if t.Tok == "specials" && t.Specials != nil {
		return t.Specials.String()
	}
	return t.Arg
}

// Options are the LatexWalker constructor options. A nil LatexContext means the default
// context database.
type Options struct {
	LatexContext    *LatexContextDb
	TolerantParsing bool
	StrictBraces    bool
}

// DefaultOptions are the Python defaults: the default context, tolerant parsing, and
// non-strict braces.
func DefaultOptions() Options { return Options{TolerantParsing: true} }

// LatexWalker parses one input string.
type LatexWalker struct {
	S                   string
	TolerantParsing     bool
	StrictBraces        bool
	DefaultParsingState *ParsingState

	r          []rune
	lineStarts []int
}

// NewLatexWalker returns a walker over s.
func NewLatexWalker(s string, opts Options) *LatexWalker {
	ctx := opts.LatexContext
	if ctx == nil {
		ctx = GetDefaultLatexContextDb()
	}
	w := &LatexWalker{S: s, TolerantParsing: opts.TolerantParsing, StrictBraces: opts.StrictBraces, r: []rune(s)}
	w.DefaultParsingState = &ParsingState{S: s, LatexContext: ctx, runes: w.r}
	return w
}

func (w *LatexWalker) defaultState() *ParsingState {
	return w.DefaultParsingState.subContext(false, false, false, "")
}

// MakeParsingState returns a copy of the default parsing state in the given mode; the
// delimiter is dropped when inMathMode is false.
func (w *LatexWalker) MakeParsingState(inMathMode bool, mathModeDelimiter string) *ParsingState {
	return w.DefaultParsingState.subContext(true, inMathMode, true, mathModeDelimiter)
}

// PosToLinenoColno returns the line (from 1) and column (from 0) of a position.
func (w *LatexWalker) PosToLinenoColno(pos int) (int, int) {
	if w.lineStarts == nil {
		w.lineStarts = []int{0}
		for i, c := range w.r {
			if c == '\n' {
				w.lineStarts = append(w.lineStarts, i+1)
			}
		}
	}
	i := sort.Search(len(w.lineStarts), func(i int) bool { return w.lineStarts[i] > pos }) - 1
	if i < 0 {
		i = 0
	}
	return 1 + i, pos - w.lineStarts[i]
}

func (w *LatexWalker) parseError(msg string, pos int) *ParseError {
	l, c := w.PosToLinenoColno(pos)
	return &ParseError{Msg: msg, S: w.S, Pos: pos, Lineno: l, Colno: c}
}

// matchEnvName matches \s*\{([\w* ._-]+)\} at s[start:], returning the name and the length
// of the match.
func matchEnvName(s []rune, start int) (string, int, bool) {
	j := start
	for j < len(s) && pyrt.IsSpace(s[j]) {
		j++
	}
	if j >= len(s) || s[j] != '{' {
		return "", 0, false
	}
	j++
	k := j
	for k < len(s) {
		c := s[k]
		if pyrt.IsWord(c) || c == '*' || c == ' ' || c == '.' || c == '_' || c == '-' {
			k++
			continue
		}
		break
	}
	if k == j || k >= len(s) || s[k] != '}' {
		return "", 0, false
	}
	return string(s[j:k]), k + 1 - start, true
}

// GetToken reads the token at pos. includeBraceChars adds (open, close) pairs to the
// braces { }, environments says whether \begin and \end make environment tokens, and a nil
// ps means the default parsing state.
func (w *LatexWalker) GetToken(pos int, includeBraceChars [][2]string, environments bool, ps *ParsingState) (*Token, error) {
	if ps == nil {
		ps = w.defaultState()
	}
	braceChars := append([][2]string{{"{", "}"}}, includeBraceChars...)
	s := w.r
	n := len(s)
	var space []rune
	if pos < 0 {
		pos = 0
	}
	for pos < n && pyrt.IsSpace(s[pos]) {
		space = append(space, s[pos])
		pos++
		if k := len(space); k >= 2 && space[k-1] == '\n' && space[k-2] == '\n' {
			return &Token{Tok: "char", Arg: "\n\n", Pos: pos - 2, Len: 2, PreSpace: string(space[:k-2])}, nil
		}
	}
	if pos >= n {
		return nil, &EndOfStreamError{FinalSpace: string(space)}
	}
	if s[pos] == '\\' {
		if pos+1 >= n {
			return nil, &EndOfStreamError{}
		}
		macro := []rune{s[pos+1]}
		alpha := false
		i := 2
		if pyrt.IsAlpha(s[pos+1]) {
			alpha = true
			for pos+i < n && pyrt.IsAlpha(s[pos+i]) {
				macro = append(macro, s[pos+i])
				i++
			}
		}
		m := string(macro)
		if m == "[" || m == "]" {
			return &Token{Tok: "mathmode_display", Arg: `\` + m, Pos: pos, Len: i, PreSpace: string(space)}, nil
		}
		if m == "(" || m == ")" {
			return &Token{Tok: "mathmode_inline", Arg: `\` + m, Pos: pos, Len: i, PreSpace: string(space)}, nil
		}
		if environments && (m == "begin" || m == "end") {
			name, mlen, ok := matchEnvName(s, pos+i)
			if !ok {
				e := w.parseError(fmt.Sprintf(`Bad \%s macro: expected {environmentname}`, m), pos)
				if w.TolerantParsing {
					return &Token{Tok: "char", Arg: `\` + m, Pos: pos, Len: i, PreSpace: string(space)}, nil
				}
				return nil, e
			}
			tok := "end_environment"
			if m == "begin" {
				tok = "begin_environment"
			}
			return &Token{Tok: tok, Arg: name, Pos: pos, Len: i + mlen, PreSpace: string(space)}, nil
		}
		var post []rune
		if alpha {
			for pos+i < n && pyrt.IsSpace(s[pos+i]) {
				post = append(post, s[pos+i])
				i++
				if k := len(post); k >= 2 && post[k-1] == '\n' && post[k-2] == '\n' {
					post = post[:k-2]
					i -= 2
					break
				}
			}
		}
		return &Token{Tok: "macro", Arg: m, Pos: pos, Len: i, PreSpace: string(space), PostSpace: string(post)}, nil
	}
	if s[pos] == '%' {
		k := pos
		for k < n && s[k] != '\n' && s[k] != '\r' {
			k++
		}
		var arglen, mlen int
		mspace := ""
		if k < n {
			j := k + 1
			for j < n && pyrt.IsSpace(s[j]) {
				j++
			}
			if j > k+1 && (s[k+1] == '\n' || s[k+1] == '\r') {
				arglen, mlen = k-pos, k-pos
			} else {
				arglen, mlen = k-pos, j-pos
				mspace = string(s[k:j])
			}
		} else {
			arglen = n - pos
			mlen = arglen
		}
		return &Token{Tok: "comment", Arg: string(sliceRunes(s, pos+1, pos+arglen)), Pos: pos, Len: mlen,
			PreSpace: string(space), PostSpace: mspace}, nil
	}
	c := string(s[pos])
	for _, bc := range braceChars {
		if c == bc[0] {
			return &Token{Tok: "brace_open", Arg: c, Pos: pos, Len: 1, PreSpace: string(space)}, nil
		}
	}
	for _, bc := range braceChars {
		if c == bc[1] {
			return &Token{Tok: "brace_close", Arg: c, Pos: pos, Len: 1, PreSpace: string(space)}, nil
		}
	}
	if s[pos] == '$' {
		if pos+1 < n && s[pos+1] == '$' && !(ps.InMathMode && ps.MathModeDelimiter == "$") {
			return &Token{Tok: "mathmode_display", Arg: "$$", Pos: pos, Len: 2, PreSpace: string(space)}, nil
		}
		return &Token{Tok: "mathmode_inline", Arg: "$", Pos: pos, Len: 1, PreSpace: string(space)}, nil
	}
	if ps.LatexContext != nil {
		if spec := ps.LatexContext.testForSpecials(s, pos); spec != nil {
			return &Token{Tok: "specials", Arg: spec.SpecialsChars, Specials: spec, Pos: pos,
				Len: pyrt.Len(spec.SpecialsChars), PreSpace: string(space)}, nil
		}
	}
	return &Token{Tok: "char", Arg: c, Pos: pos, Len: 1, PreSpace: string(space)}, nil
}

// GetLatexExpression reads a single LaTeX expression at pos: a character, a macro without
// its arguments, specials, or a braced group. It returns the node with its position and
// length.
func (w *LatexWalker) GetLatexExpression(pos int, ps *ParsingState) (Node, int, int, error) {
	return w.getLatexExpression(pos, w.StrictBraces, ps)
}

func (w *LatexWalker) getLatexExpression(pos int, strictBraces bool, ps *ParsingState) (Node, int, int, error) {
	if ps == nil {
		ps = w.defaultState()
	}
	tok, err := w.GetToken(pos, nil, false, ps)
	if err != nil {
		return nil, 0, 0, err
	}
	base := func(p, l int) NodeBase { return NodeBase{ParsingState: ps, Pos: p, Len: l} }
	switch tok.Tok {
	case "macro":
		if tok.Arg == "end" {
			if !w.TolerantParsing {
				return nil, 0, 0, w.parseError(`Expected expression, got \end`, pos)
			}
			return &CharsNode{NodeBase: base(tok.Pos, 0)}, tok.Pos, 0, nil
		}
		return &MacroNode{NodeBase: base(tok.Pos, tok.Len), MacroName: tok.Arg, MacroPostSpace: tok.PostSpace}, tok.Pos, tok.Len, nil
	case "specials":
		return &SpecialsNode{NodeBase: base(tok.Pos, tok.Len), SpecialsChars: tok.Specials.SpecialsChars}, tok.Pos, tok.Len, nil
	case "comment":
		return w.getLatexExpression(tok.Pos+tok.Len, strictBraces, ps)
	case "brace_open":
		g, p, l, err := w.GetLatexBracedGroup(tok.Pos, "{", ps)
		if err != nil {
			return nil, 0, 0, err
		}
		return g, p, l, nil
	case "brace_close":
		if strictBraces && !w.TolerantParsing {
			return nil, 0, 0, w.parseError(fmt.Sprintf("Expected expression, got closing brace '%s'", tok.Arg), pos)
		}
		return &CharsNode{NodeBase: base(tok.Pos, 0)}, tok.Pos, 0, nil
	case "char":
		return &CharsNode{NodeBase: base(tok.Pos, tok.Len), Chars: tok.Arg}, tok.Pos, tok.Len, nil
	case "mathmode_inline", "mathmode_display":
		if strings.HasPrefix(tok.Arg, `\`) {
			return &MacroNode{NodeBase: base(tok.Pos, tok.Len), MacroName: tok.Arg, Nodeargd: &ParsedMacroArgs{Argnlist: []Node{}},
				MacroPostSpace: tok.PostSpace}, tok.Pos, tok.Len, nil
		}
		return &CharsNode{NodeBase: base(tok.Pos, tok.Len), Chars: tok.Arg}, tok.Pos, tok.Len, nil
	}
	return nil, 0, 0, w.parseError(fmt.Sprintf("Unknown token type: %s", tok.Tok), pos)
}

// GetLatexMaybeOptionalArg reads an optional argument [...] at pos; the group is nil when
// there is none.
func (w *LatexWalker) GetLatexMaybeOptionalArg(pos int, ps *ParsingState) (*GroupNode, int, int, error) {
	if ps == nil {
		ps = w.defaultState()
	}
	tok, err := w.GetToken(pos, [][2]string{{"[", "]"}}, false, ps)
	if err != nil {
		if _, ok := err.(*EndOfStreamError); ok {
			return nil, 0, 0, nil
		}
		return nil, 0, 0, err
	}
	if tok.Tok == "brace_open" && tok.Arg == "[" {
		return w.GetLatexBracedGroup(pos, "[", ps)
	}
	return nil, 0, 0, nil
}

type closingBrace struct {
	open, close string
}

// GetLatexBracedGroup reads a group delimited by braceType ("{", "[", "(", "<", or a
// two-character string giving the opening and closing delimiters) at pos.
func (w *LatexWalker) GetLatexBracedGroup(pos int, braceType string, ps *ParsingState) (*GroupNode, int, int, error) {
	if ps == nil {
		ps = w.defaultState()
	}
	var closing string
	switch braceType {
	case "{":
		closing = "}"
	case "[":
		closing = "]"
	case "(":
		closing = ")"
	case "<":
		closing = ">"
	default:
		bt := []rune(braceType)
		if len(bt) != 2 {
			return nil, 0, 0, valueError(fmt.Sprintf("Invalid brace type for get_latex_braced_group(): %s", braceType))
		}
		braceType, closing = string(bt[0]), string(bt[1])
	}
	var include [][2]string
	if braceType != "" && braceType != "{" {
		include = [][2]string{{braceType, closing}}
	}
	first, err := w.GetToken(pos, include, true, ps)
	if err != nil {
		return nil, 0, 0, err
	}
	if first.Tok != "brace_open" || first.Arg != braceType {
		at := ""
		if pos >= 0 && pos < len(w.r) {
			at = string(w.r[pos])
		}
		return nil, 0, 0, w.parseError(fmt.Sprintf("get_latex_braced_group: not an opening brace/bracket: %s", at), pos)
	}
	nodelist, npos, nlen, err := w.getLatexNodes(first.Pos+first.Len, nodesStop{brace: &closingBrace{braceType, closing}}, 0, ps)
	if err != nil {
		return nil, 0, 0, err
	}
	g := &GroupNode{NodeBase: NodeBase{ParsingState: ps, Pos: first.Pos, Len: npos + nlen - first.Pos},
		Nodelist: nodelist, Delimiters: [2]string{braceType, closing}}
	return g, g.Pos, g.Len, nil
}

// GetLatexEnvironment reads an environment at pos; when environmentName is not "" the
// environment must have that name.
func (w *LatexWalker) GetLatexEnvironment(pos int, environmentName string, ps *ParsingState) (*EnvironmentNode, int, int, error) {
	if ps == nil {
		ps = w.defaultState()
	}
	startpos := pos
	first, err := w.GetToken(pos, nil, true, ps)
	if err != nil {
		return nil, 0, 0, err
	}
	if first.Tok != "begin_environment" || (environmentName != "" && first.Arg != environmentName) {
		want := environmentName
		if want == "" {
			want = "<environment name>"
		}
		return nil, 0, 0, w.parseError(fmt.Sprintf(`get_latex_environment: expected \begin{%s}: %s`, want, first.argStr()), pos)
	}
	if environmentName == "" {
		environmentName = first.Arg
	}
	pos = first.Pos + first.Len
	spec := ps.LatexContext.GetEnvironmentSpec(environmentName)
	if spec == nil {
		spec = &EnvironmentSpec{}
	}
	argd, apos, alen, err := spec.parser().ParseArgs(w, pos, ps)
	if err == nil && argd == nil {
		err = typeError("object of type 'NoneType' has no len()")
	}
	if err != nil {
		if !isParseFailure(err) {
			return nil, 0, 0, err
		}
		if e := w.exchandle(err, first, fmt.Sprintf(`arguments of environment "\begin{%s}"`, environmentName)); e != nil {
			return nil, 0, 0, e
		}
		argd, apos, alen = nil, pos, 0
	}
	pos = apos + alen
	inner := ps
	if spec.IsMathMode {
		inner = ps.subContext(true, true, true, "{"+environmentName+"}")
	}
	nodelist, npos, nlen, err := w.getLatexNodes(pos, nodesStop{endEnv: environmentName}, 0, inner)
	if err != nil {
		return nil, 0, 0, err
	}
	optarg, args := argd.legacy()
	env := &EnvironmentNode{NodeBase: NodeBase{ParsingState: ps, Pos: startpos, Len: npos + nlen - startpos},
		EnvironmentName: environmentName, Nodelist: nodelist, Nodeargd: argd, OptArgs: []Node{optarg}, Args: args}
	return env, env.Pos, env.Len, nil
}

func isParseFailure(err error) bool {
	switch err.(type) {
	case *EndOfStreamError, *ParseError:
		return true
	}
	return false
}

// exchandle is _exchandle_parse_subexpression: it turns an end of stream into a parse
// error, records the open context, and returns nil in tolerant mode.
func (w *LatexWalker) exchandle(err error, tok *Token, what string) error {
	var pe *ParseError
	switch e := err.(type) {
	case *EndOfStreamError:
		pe = w.parseError("End of input while parsing "+what, tok.Pos)
	case *ParseError:
		pe = e
	default:
		return err
	}
	if pe.Lineno == 0 {
		pe.Lineno, pe.Colno = w.PosToLinenoColno(pe.Pos)
	}
	l, c := w.PosToLinenoColno(tok.Pos)
	pe.OpenContexts = append(pe.OpenContexts, OpenContext{What: what, Pos: tok.Pos, Lineno: l, Colno: c})
	if w.TolerantParsing {
		return nil
	}
	return pe
}

// NodesOptions are the stop conditions of GetLatexNodes. StopUponClosingBrace is "}", "]",
// ")", ">", or a two-character string of opening and closing delimiters; ReadMaxNodes 0
// means no limit; a nil ParsingState means the default one.
type NodesOptions struct {
	StopUponClosingBrace    string
	StopUponEndEnvironment  string
	StopUponClosingMathmode string
	ReadMaxNodes            int
	ParsingState            *ParsingState
}

type nodesStop struct {
	brace       *closingBrace
	endEnv      string
	closingMath string
}

// GetLatexNodes reads nodes from pos until the input ends or a stop condition is met, and
// returns them with the position and length read.
func (w *LatexWalker) GetLatexNodes(pos int, opts NodesOptions) ([]Node, int, int, error) {
	stop := nodesStop{endEnv: opts.StopUponEndEnvironment, closingMath: opts.StopUponClosingMathmode}
	if b := opts.StopUponClosingBrace; b != "" {
		switch b {
		case "}":
			stop.brace = &closingBrace{"{", "}"}
		case "]":
			stop.brace = &closingBrace{"[", "]"}
		case ")":
			stop.brace = &closingBrace{"(", ")"}
		case ">":
			stop.brace = &closingBrace{"<", ">"}
		default:
			if br := []rune(b); len(br) == 2 {
				stop.brace = &closingBrace{string(br[0]), string(br[1])}
			} else {
				stop.brace = &closingBrace{"", b}
			}
		}
	}
	return w.getLatexNodes(pos, stop, opts.ReadMaxNodes, opts.ParsingState)
}

type posPointer struct {
	pos          int
	ps           *ParsingState
	lastchars    string
	lastcharsPos int
	hasPos       bool
}

func (p *posPointer) push(pos int, chars string) {
	p.lastchars += chars
	if !p.hasPos {
		p.lastcharsPos = pos
		p.hasPos = true
	}
}

func (p *posPointer) flush() (int, string) {
	pos, chars := p.lastcharsPos, p.lastchars
	p.lastchars = ""
	p.hasPos = false
	return pos, chars
}

func (w *LatexWalker) getLatexNodes(pos int, stop nodesStop, readMaxNodes int, ps *ParsingState) ([]Node, int, int, error) {
	if ps == nil {
		ps = w.defaultState()
	}
	nodelist := []Node{}
	var include [][2]string
	if stop.brace != nil && stop.brace.close != "}" {
		include = [][2]string{{stop.brace.open, stop.brace.close}}
	}
	origpos := pos
	p := &posPointer{pos: pos, ps: ps}
	full := func() bool { return readMaxNodes > 0 && len(nodelist) >= readMaxNodes }

	// doRead reads one token and returns whether to stop, with the end of stream that
	// stopped it in tolerant mode.
	doRead := func() (bool, *EndOfStreamError, error) {
		tok, err := w.GetToken(p.pos, include, true, p.ps)
		if err != nil {
			if e, ok := err.(*EndOfStreamError); ok && w.TolerantParsing {
				return true, e, nil
			}
			return false, nil, err
		}
		p.pos = tok.Pos + tok.Len
		if tok.Tok == "char" {
			p.push(tok.Pos-pyrt.Len(tok.PreSpace), tok.PreSpace+tok.Arg)
			return false, nil, nil
		}
		if p.lastchars != "" {
			charspos, chars := p.flush()
			nodelist = append(nodelist, &CharsNode{NodeBase: NodeBase{ParsingState: p.ps, Pos: charspos, Len: tok.Pos - charspos},
				Chars: chars + tok.PreSpace})
			if full() {
				p.pos = tok.Pos
				return true, nil, nil
			}
		} else if tok.PreSpace != "" {
			l := pyrt.Len(tok.PreSpace)
			nodelist = append(nodelist, &CharsNode{NodeBase: NodeBase{ParsingState: p.ps, Pos: tok.Pos - l, Len: l}, Chars: tok.PreSpace})
			if full() {
				p.pos = tok.Pos
				return true, nil, nil
			}
		}
		at := func(n Node) (bool, *EndOfStreamError, error) {
			nodelist = append(nodelist, n)
			return full(), nil, nil
		}
		switch tok.Tok {
		case "brace_close":
			if stop.brace == nil || tok.Arg != stop.brace.close {
				return false, nil, w.parseError(fmt.Sprintf("Unexpected mismatching closing brace: '%s'", tok.Arg), tok.Pos)
			}
			return true, nil, nil
		case "end_environment":
			if stop.endEnv == "" {
				return false, nil, w.parseError(fmt.Sprintf("Unexpected closing environment: '%s'", tok.Arg), tok.Pos)
			}
			if tok.Arg != stop.endEnv {
				return false, nil, w.parseError(fmt.Sprintf("Unexpected mismatching closing environment: '%s', was expecting '%s'",
					tok.Arg, stop.endEnv), tok.Pos)
			}
			return true, nil, nil
		case "mathmode_inline", "mathmode_display":
			if stop.closingMath != "" {
				if tok.Arg == stop.closingMath {
					return true, nil, nil
				}
				if tok.Arg == `\)` || tok.Arg == `\]` {
					return false, nil, w.parseError(fmt.Sprintf("Mismatching closing math mode: '%s', expected '%s'",
						tok.Arg, stop.closingMath), tok.Pos)
				}
			} else if tok.Arg == `\)` || tok.Arg == `\]` {
				return false, nil, w.parseError(fmt.Sprintf("Unexpected closing math mode: '%s'", tok.Arg), tok.Pos)
			}
			closing := tok.Arg
			switch tok.Arg {
			case `\(`:
				closing = `\)`
			case `\[`:
				closing = `\]`
			}
			displaytype := "display"
			if tok.Arg == `\(` || tok.Arg == "$" {
				displaytype = "inline"
			}
			inner := p.ps.subContext(true, true, true, tok.Arg)
			ml, mpos, mlen, err := w.getLatexNodes(p.pos, nodesStop{closingMath: closing}, 0, inner)
			if err != nil {
				w.addContext(err, fmt.Sprintf(`math mode "%s"`, tok.Arg), tok.Pos)
				return false, nil, err
			}
			p.pos = mpos + mlen
			return at(&MathNode{NodeBase: NodeBase{ParsingState: p.ps, Pos: tok.Pos, Len: mpos + mlen - tok.Pos},
				Displaytype: displaytype, Nodelist: ml, Delimiters: [2]string{tok.Arg, closing}})
		case "comment":
			return at(&CommentNode{NodeBase: NodeBase{ParsingState: p.ps, Pos: tok.Pos, Len: tok.Len},
				Comment: tok.Arg, CommentPostSpace: tok.PostSpace})
		case "brace_open":
			g, bpos, blen, err := w.GetLatexBracedGroup(tok.Pos, tok.Arg, p.ps)
			if err != nil {
				w.addContext(err, "open brace", tok.Pos)
				return false, nil, err
			}
			p.pos = bpos + blen
			return at(g)
		case "begin_environment":
			env, epos, elen, err := w.GetLatexEnvironment(tok.Pos, tok.Arg, p.ps)
			if err != nil {
				w.addContext(err, fmt.Sprintf(`begin environment "%s"`, tok.Arg), tok.Pos)
				return false, nil, err
			}
			p.pos = epos + elen
			return at(env)
		case "macro":
			spec := p.ps.LatexContext.GetMacroSpec(tok.Arg)
			if spec == nil {
				spec = &MacroSpec{}
			}
			argd, apos, alen, err := spec.parser().ParseArgs(w, tok.Pos+tok.Len, p.ps)
			if err == nil && argd == nil {
				return false, nil, typeError("object of type 'NoneType' has no len()")
			}
			if err != nil {
				if !isParseFailure(err) {
					return false, nil, err
				}
				if e := w.exchandle(err, tok, fmt.Sprintf(`arguments of macro "%s"`, tok.Arg)); e != nil {
					return false, nil, e
				}
				argd, apos, alen = nil, tok.Pos+tok.Len, 0
			}
			p.pos = apos + alen
			optarg, args := argd.legacy()
			return at(&MacroNode{NodeBase: NodeBase{ParsingState: p.ps, Pos: tok.Pos, Len: p.pos - tok.Pos},
				MacroName: tok.Arg, Nodeargd: argd, MacroPostSpace: tok.PostSpace, NodeOptArg: optarg, NodeArgs: args})
		case "specials":
			spec := tok.Specials
			p.pos = tok.Pos + tok.Len
			var argd *ParsedMacroArgs
			if spec.ArgsParser != nil {
				a, apos, alen, err := spec.ArgsParser.ParseArgs(w, p.pos, p.ps)
				if err != nil {
					if !isParseFailure(err) {
						return false, nil, err
					}
					if e := w.exchandle(err, tok, fmt.Sprintf(`arguments of specials "%s"`, spec.SpecialsChars)); e != nil {
						return false, nil, e
					}
					a, apos, alen = nil, p.pos, 0
				}
				if a != nil || err != nil {
					argd = a
					p.pos = apos + alen
				}
			}
			return at(&SpecialsNode{NodeBase: NodeBase{ParsingState: p.ps, Pos: tok.Pos, Len: p.pos - tok.Pos},
				SpecialsChars: spec.SpecialsChars, Nodeargd: argd})
		}
		return false, nil, w.parseError(fmt.Sprintf("Unknown token: %s", tok.Tok), p.pos)
	}

	for {
		stopNow, eos, err := doRead()
		if err != nil {
			switch e := err.(type) {
			case *EndOfStreamError:
				if stop.brace != nil || stop.endEnv != "" || stop.closingMath != "" {
					var expecting string
					switch {
					case stop.brace != nil:
						expecting = "'" + stop.brace.close + "'"
					case stop.endEnv != "":
						expecting = `\end{` + stop.endEnv + "}"
					default:
						expecting = "'" + stop.closingMath + "'"
					}
					l, c := w.PosToLinenoColno(len(w.r))
					pe := &ParseError{Msg: "Unexpected end of stream, was expecting " + expecting, S: w.S, Pos: p.pos, Lineno: l, Colno: c}
					if !w.TolerantParsing {
						return nil, 0, 0, pe
					}
					stopNow = true
				} else {
					eos, stopNow = e, true
				}
			case *ParseError:
				if !w.TolerantParsing {
					return nil, 0, 0, e
				}
				stopNow = false
			default:
				return nil, 0, 0, err
			}
		}
		if stopNow {
			if eos != nil {
				p.push(p.pos, eos.FinalSpace)
				p.pos += pyrt.Len(eos.FinalSpace)
			}
			if p.lastchars != "" {
				charspos, chars := p.flush()
				nodelist = append(nodelist, &CharsNode{NodeBase: NodeBase{ParsingState: p.ps, Pos: charspos, Len: pyrt.Len(chars)}, Chars: chars})
			}
			return nodelist, origpos, p.pos - origpos, nil
		}
	}
}

func (w *LatexWalker) addContext(err error, what string, pos int) {
	if pe, ok := err.(*ParseError); ok {
		l, c := w.PosToLinenoColno(pos)
		pe.OpenContexts = append(pe.OpenContexts, OpenContext{What: what, Pos: pos, Lineno: l, Colno: c})
	}
}
