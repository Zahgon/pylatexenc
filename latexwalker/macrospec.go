package latexwalker

import (
	"fmt"
	"strings"

	"github.com/phfaist/pylatexenc"
	"github.com/phfaist/pylatexenc/internal/ctxdb"
	"github.com/phfaist/pylatexenc/internal/pyrt"
)

// ArgsParser parses the arguments that follow a macro, the \begin{...} of an environment, or
// specials. It returns the parsed arguments and the position and length they span; a nil
// result with a nil error stands for Python's None.
type ArgsParser interface {
	ParseArgs(w *LatexWalker, pos int, ps *ParsingState) (*ParsedMacroArgs, int, int, error)
	String() string
}

// ArgMathMode says in which mode one argument is parsed.
type ArgMathMode int

const (
	// MathModeInherit parses the argument in the surrounding mode (Python's None).
	MathModeInherit ArgMathMode = iota
	// MathModeOn parses the argument in math mode (True).
	MathModeOn
	// MathModeOff parses the argument in text mode (False).
	MathModeOff
)

func (m ArgMathMode) String() string {
	switch m {
	case MathModeOn:
		return "True"
	case MathModeOff:
		return "False"
	}
	return "None"
}

// MacroStandardArgsParser parses arguments described by an argspec string of '*' (an
// optional star), '[' (an optional argument) and '{' (a mandatory argument).
type MacroStandardArgsParser struct {
	Argspec            string
	OptionalArgNoSpace bool
	ArgsMathMode       []ArgMathMode // nil when None
}

// NewMacroStandardArgsParser checks the argspec as the Python constructor does.
func NewMacroStandardArgsParser(argspec string, optionalArgNoSpace bool, argsMathMode []ArgMathMode) (*MacroStandardArgsParser, error) {
	for _, c := range argspec {
		if c != '*' && c != '[' && c != '{' {
			return nil, &pylatexenc.BuiltinError{Class: "TypeError", Message: fmt.Sprintf(
				"argspec must be a string containing chars '*', '[', '{' only: %s", pyrt.Repr(argspec))}
		}
	}
	return &MacroStandardArgsParser{Argspec: argspec, OptionalArgNoSpace: optionalArgNoSpace, ArgsMathMode: argsMathMode}, nil
}

func reprMathModes(m []ArgMathMode) string {
	if m == nil {
		return "None"
	}
	parts := make([]string, len(m))
	for i, x := range m {
		parts[i] = x.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func pyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func (p *MacroStandardArgsParser) String() string {
	return fmt.Sprintf("MacroStandardArgsParser(argspec=%s, optional_arg_no_space=%s, args_math_mode=%s)",
		pyrt.Repr(p.Argspec), pyBool(p.OptionalArgNoSpace), reprMathModes(p.ArgsMathMode))
}

// ParseArgs implements ArgsParser.
func (p *MacroStandardArgsParser) ParseArgs(w *LatexWalker, pos int, ps *ParsingState) (*ParsedMacroArgs, int, int, error) {
	if ps == nil {
		ps = w.defaultState()
	}
	spec := []rune(p.Argspec)
	if p.ArgsMathMode != nil && len(p.ArgsMathMode) != len(spec) {
		return nil, 0, 0, &pylatexenc.BuiltinError{Class: "ValueError", Message: fmt.Sprintf(
			"Invalid args_math_mode=%s for argspec=%s!", reprMathModes(p.ArgsMathMode), pyrt.Repr(p.Argspec))}
	}
	inner := func(j int) *ParsingState {
		if p.ArgsMathMode == nil {
			return ps
		}
		amm := p.ArgsMathMode[j]
		if amm == MathModeInherit || (amm == MathModeOn) == ps.InMathMode {
			return ps
		}
		return ps.subContext(true, amm == MathModeOn, false, "")
	}
	argnlist := []Node{}
	at := pos
	for j, argt := range spec {
		switch argt {
		case '{':
			node, np, nl, err := w.getLatexExpression(at, false, inner(j))
			if err != nil {
				return nil, 0, 0, err
			}
			at = np + nl
			argnlist = append(argnlist, node)
		case '[':
			if p.OptionalArgNoSpace && at < len(w.r) && at >= 0 && pyrt.IsSpace(w.r[at]) {
				argnlist = append(argnlist, nil)
				continue
			}
			g, np, nl, err := w.GetLatexMaybeOptionalArg(at, inner(j))
			if err != nil {
				return nil, 0, 0, err
			}
			if g == nil {
				argnlist = append(argnlist, nil)
				continue
			}
			at = np + nl
			argnlist = append(argnlist, g)
		case '*':
			tok, err := w.GetToken(at, nil, true, nil)
			if err != nil {
				return nil, 0, 0, err
			}
			if tok.Tok == "char" && strings.HasPrefix(tok.Arg, "*") {
				argnlist = append(argnlist, &CharsNode{NodeBase: NodeBase{ParsingState: inner(j), Pos: tok.Pos, Len: 1}, Chars: "*"})
				at = tok.Pos + 1
			} else {
				argnlist = append(argnlist, nil)
			}
		default:
			return nil, 0, 0, &Error{Msg: fmt.Sprintf("Unknown macro argument kind for macro: %s", pyrt.Repr(string(argt)))}
		}
	}
	return &ParsedMacroArgs{Argspec: p.Argspec, Argnlist: argnlist}, pos, at - pos, nil
}

// VerbatimArgsParser reads a verbatim argument: VerbatimArgType "verbatim-environment" reads
// up to the next \end{verbatim}, and "verb-macro" reads a \verb argument between two
// copies of its delimiter.
type VerbatimArgsParser struct {
	VerbatimArgType string
}

func (p *VerbatimArgsParser) String() string {
	return fmt.Sprintf("VerbatimArgsParser(verbatim_arg_type=%s)", pyrt.Repr(p.VerbatimArgType))
}

// ParseArgs implements ArgsParser.
func (p *VerbatimArgsParser) ParseArgs(w *LatexWalker, pos int, ps *ParsingState) (*ParsedMacroArgs, int, int, error) {
	s := w.r
	switch p.VerbatimArgType {
	case "verbatim-environment":
		end := indexRunes(s, []rune(`\end{verbatim}`), pos)
		if end == -1 {
			return nil, 0, 0, &ParseError{S: w.S, Pos: pos, Msg: `Cannot find matching \end{verbatim}`}
		}
		n := end - pos
		node := &CharsNode{NodeBase: NodeBase{ParsingState: ps, Pos: pos, Len: n}, Chars: string(sliceRunes(s, pos, pos+n))}
		return &ParsedMacroArgs{Argspec: "{", Argnlist: []Node{node}, Verbatim: &VerbatimArgs{Text: node.Chars}}, pos, n, nil
	case "verb-macro":
		if pos < 0 || pos >= len(s) {
			return nil, 0, 0, &pylatexenc.BuiltinError{Class: "IndexError", Message: "string index out of range"}
		}
		for pyrt.IsSpace(s[pos]) {
			pos++
			if pos >= len(s) {
				return nil, 0, 0, &ParseError{S: w.S, Pos: pos, Msg: `Missing argument to \verb command`}
			}
		}
		delim := s[pos]
		begin := pos + 1
		end := indexRunes(s, []rune{delim}, begin)
		if end == -1 {
			return nil, 0, 0, &ParseError{S: w.S, Pos: pos, Msg: `End of stream reached while reading argument to \verb command`}
		}
		node := &CharsNode{NodeBase: NodeBase{ParsingState: ps, Pos: begin, Len: end - begin}, Chars: string(s[begin:end])}
		d := string(delim)
		return &ParsedMacroArgs{Argspec: "{", Argnlist: []Node{node},
			Verbatim: &VerbatimArgs{Text: node.Chars, Delimiters: [2]string{d, d}}}, pos, end + 1 - pos, nil
	}
	return nil, 0, 0, nil
}

func indexRunes(s, sub []rune, from int) int {
	if from < 0 {
		from = 0
	}
	for i := from; i+len(sub) <= len(s); i++ {
		match := true
		for j, r := range sub {
			if s[i+j] != r {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

var emptyArgsParser = &MacroStandardArgsParser{}

// MacroSpec is the specification of a macro. A nil ArgsParser means a
// MacroStandardArgsParser with an empty argspec.
type MacroSpec struct {
	MacroName  string
	ArgsParser ArgsParser
}

func (m *MacroSpec) parser() ArgsParser {
	if m.ArgsParser == nil {
		return emptyArgsParser
	}
	return m.ArgsParser
}

func (m *MacroSpec) String() string {
	return fmt.Sprintf("MacroSpec(macroname=%s, args_parser=%s)", pyrt.Repr(m.MacroName), m.parser().String())
}

// EnvironmentSpec is the specification of an environment. A nil ArgsParser means a
// MacroStandardArgsParser with an empty argspec.
type EnvironmentSpec struct {
	EnvironmentName string
	ArgsParser      ArgsParser
	IsMathMode      bool
}

func (e *EnvironmentSpec) parser() ArgsParser {
	if e.ArgsParser == nil {
		return emptyArgsParser
	}
	return e.ArgsParser
}

func (e *EnvironmentSpec) String() string {
	return fmt.Sprintf("EnvironmentSpec(environmentname=%s, args_parser=%s, is_math_mode=%s)",
		pyrt.Repr(e.EnvironmentName), e.parser().String(), pyBool(e.IsMathMode))
}

// SpecialsSpec is the specification of specials: characters with a meaning of their own,
// such as "~" or "--". A nil ArgsParser means the specials take no arguments.
type SpecialsSpec struct {
	SpecialsChars string
	ArgsParser    ArgsParser
}

func (s *SpecialsSpec) String() string {
	ap := "None"
	if s.ArgsParser != nil {
		ap = s.ArgsParser.String()
	}
	return fmt.Sprintf("SpecialsSpec(specials_chars=%s, args_parser=%s)", pyrt.Repr(s.SpecialsChars), ap)
}

func stdArgspec(optarg bool, numargs int) string {
	spec := ""
	if optarg {
		spec = "["
	}
	return spec + strings.Repeat("{", numargs)
}

// StdMacro is std_macro(name, optarg, numargs).
func StdMacro(name string, optarg bool, numargs int) *MacroSpec {
	return &MacroSpec{MacroName: name, ArgsParser: &MacroStandardArgsParser{Argspec: stdArgspec(optarg, numargs)}}
}

// StdEnvironment is std_environment(name, optarg, numargs, is_math_mode=isMathMode).
func StdEnvironment(name string, optarg bool, numargs int, isMathMode bool) *EnvironmentSpec {
	return &EnvironmentSpec{EnvironmentName: name, ArgsParser: &MacroStandardArgsParser{Argspec: stdArgspec(optarg, numargs)}, IsMathMode: isMathMode}
}

// StdSpecials is std_specials(chars).
func StdSpecials(chars string) *SpecialsSpec { return &SpecialsSpec{SpecialsChars: chars} }

// Placement says where AddContextCategory puts a new category: first (Prepend), before an
// existing category (InsertBefore, or first when that category is not registered), after
// one (InsertAfter, or last when it is not registered), or last when nothing is set.
type Placement = ctxdb.Placement

type specDb = ctxdb.Db[*MacroSpec, *EnvironmentSpec, *SpecialsSpec]

var specKeys = ctxdb.Keys[*MacroSpec, *EnvironmentSpec, *SpecialsSpec]{
	Macro:       func(m *MacroSpec) string { return m.MacroName },
	Environment: func(e *EnvironmentSpec) string { return e.EnvironmentName },
	Specials:    func(s *SpecialsSpec) string { return s.SpecialsChars },
}

// LatexContextDb is the database of macro, environment and specials specs the walker
// consults, organised in categories.
type LatexContextDb struct {
	db *specDb
}

// NewLatexContextDb returns an empty database.
func NewLatexContextDb() *LatexContextDb { return &LatexContextDb{db: ctxdb.New(specKeys)} }

// AddContextCategory is add_context_category().
func (c *LatexContextDb) AddContextCategory(category string, macros []*MacroSpec, environments []*EnvironmentSpec, specials []*SpecialsSpec, at Placement) error {
	return c.db.AddCategory(category, macros, environments, specials, at)
}

// SetUnknownMacroSpec sets the spec returned for macros no category defines.
func (c *LatexContextDb) SetUnknownMacroSpec(spec *MacroSpec) { c.db.UnknownMacro = spec }

// SetUnknownEnvironmentSpec sets the spec returned for undefined environments.
func (c *LatexContextDb) SetUnknownEnvironmentSpec(spec *EnvironmentSpec) {
	c.db.UnknownEnvironment = spec
}

// SetUnknownSpecialsSpec sets the spec GetSpecialsSpec returns for undefined specials.
func (c *LatexContextDb) SetUnknownSpecialsSpec(spec *SpecialsSpec) { c.db.UnknownSpecials = spec }

// Categories returns the category names in lookup order.
func (c *LatexContextDb) Categories() []string { return c.db.Categories() }

// GetMacroSpec returns the spec of a macro, or the unknown-macro spec (nil by default).
func (c *LatexContextDb) GetMacroSpec(name string) *MacroSpec { return c.db.Macro(name) }

// GetEnvironmentSpec returns the spec of an environment, or the unknown-environment spec.
func (c *LatexContextDb) GetEnvironmentSpec(name string) *EnvironmentSpec {
	return c.db.Environment(name)
}

// GetSpecialsSpec returns the spec of the specials with exactly these characters, or the
// unknown-specials spec.
func (c *LatexContextDb) GetSpecialsSpec(chars string) *SpecialsSpec { return c.db.SpecialsSpec(chars) }

// TestForSpecials returns the specials that start s at code point pos, or nil.
func (c *LatexContextDb) TestForSpecials(s string, pos int) *SpecialsSpec {
	return c.testForSpecials([]rune(s), pos)
}

func (c *LatexContextDb) testForSpecials(s []rune, pos int) *SpecialsSpec {
	spec, ok := c.db.TestForSpecials(s, pos)
	if !ok {
		return nil
	}
	return spec
}

// IterMacroSpecs lists the macro specs of the given categories in order, or of all
// categories when categories is nil.
func (c *LatexContextDb) IterMacroSpecs(categories []string) ([]*MacroSpec, error) {
	return c.db.MacroSpecs(categories)
}

// IterEnvironmentSpecs lists environment specs like IterMacroSpecs.
func (c *LatexContextDb) IterEnvironmentSpecs(categories []string) ([]*EnvironmentSpec, error) {
	return c.db.EnvironmentSpecs(categories)
}

// IterSpecialsSpecs lists specials specs like IterMacroSpecs.
func (c *LatexContextDb) IterSpecialsSpecs(categories []string) ([]*SpecialsSpec, error) {
	return c.db.SpecialsSpecs(categories)
}

// FilterContext is filter_context(): a new database with the categories kept (all when
// keepCategories is empty) minus the ones excluded, holding only the kinds of spec named in
// keepWhich ("macros", "environments", "specials"; all when empty).
func (c *LatexContextDb) FilterContext(keepCategories, excludeCategories, keepWhich []string) *LatexContextDb {
	return &LatexContextDb{db: c.db.Filter(keepCategories, excludeCategories, keepWhich)}
}

// GetDefaultLatexContextDb returns a new database holding pylatexenc's default specs.
func GetDefaultLatexContextDb() *LatexContextDb {
	c := NewLatexContextDb()
	for _, cat := range defaultSpecs() {
		_ = c.AddContextCategory(cat.name, cat.macros, cat.environments, cat.specials, Placement{})
	}
	return c
}

type specCategory struct {
	name         string
	macros       []*MacroSpec
	environments []*EnvironmentSpec
	specials     []*SpecialsSpec
}
