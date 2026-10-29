package latex2text

import (
	"github.com/phfaist/pylatexenc/internal/ctxdb"
	"github.com/phfaist/pylatexenc/latexwalker"
)

// ReplFunc computes the replacement text of a node. It plays the part of a callable
// simplify_repl.
type ReplFunc func(n latexwalker.Node, l2t *LatexNodes2Text) (string, error)

// MacroTextSpec says how a macro converts to text: SimplifyFunc, when set, or else a
// non-empty SimplifyRepl gives the replacement; otherwise the macro is dropped when Discard
// is true, and replaced by the text of its arguments when it is false.
type MacroTextSpec struct {
	MacroName    string
	SimplifyRepl string
	SimplifyFunc ReplFunc
	Discard      bool
}

// NewMacroTextSpec is MacroTextSpec(name, simplify_repl): Discard is true, the Python default.
func NewMacroTextSpec(name, simplifyRepl string) *MacroTextSpec {
	return &MacroTextSpec{MacroName: name, SimplifyRepl: simplifyRepl, Discard: true}
}

// EnvironmentTextSpec says how an environment converts to text, like MacroTextSpec, except
// that the text of the environment's body replaces it when there is no replacement and
// Discard is false.
type EnvironmentTextSpec struct {
	EnvironmentName string
	SimplifyRepl    string
	SimplifyFunc    ReplFunc
	Discard         bool
}

// NewEnvironmentTextSpec is EnvironmentTextSpec(name, simplify_repl): Discard is false.
func NewEnvironmentTextSpec(name, simplifyRepl string) *EnvironmentTextSpec {
	return &EnvironmentTextSpec{EnvironmentName: name, SimplifyRepl: simplifyRepl}
}

// SpecialsTextSpec says how specials convert to text. Like Python's class it has no discard
// setting, so a spec without a replacement fails when it is used.
type SpecialsTextSpec struct {
	SpecialsChars string
	SimplifyRepl  string
	SimplifyFunc  ReplFunc
}

// Placement says where AddContextCategory puts a new category; see latexwalker.Placement.
type Placement = ctxdb.Placement

type textDb = ctxdb.Db[*MacroTextSpec, *EnvironmentTextSpec, *SpecialsTextSpec]

var textKeys = ctxdb.Keys[*MacroTextSpec, *EnvironmentTextSpec, *SpecialsTextSpec]{
	Macro:       func(m *MacroTextSpec) string { return m.MacroName },
	Environment: func(e *EnvironmentTextSpec) string { return e.EnvironmentName },
	Specials:    func(s *SpecialsTextSpec) string { return s.SpecialsChars },
}

// LatexContextDb is the database of text specs, organised in categories exactly like
// latexwalker.LatexContextDb.
type LatexContextDb struct {
	db *textDb
}

// NewLatexContextDb returns an empty database.
func NewLatexContextDb() *LatexContextDb { return &LatexContextDb{db: ctxdb.New(textKeys)} }

// AddContextCategory is add_context_category().
func (c *LatexContextDb) AddContextCategory(category string, macros []*MacroTextSpec, environments []*EnvironmentTextSpec, specials []*SpecialsTextSpec, at Placement) error {
	return c.db.AddCategory(category, macros, environments, specials, at)
}

// SetUnknownMacroSpec sets the spec returned for macros no category defines.
func (c *LatexContextDb) SetUnknownMacroSpec(spec *MacroTextSpec) { c.db.UnknownMacro = spec }

// SetUnknownEnvironmentSpec sets the spec returned for undefined environments.
func (c *LatexContextDb) SetUnknownEnvironmentSpec(spec *EnvironmentTextSpec) {
	c.db.UnknownEnvironment = spec
}

// SetUnknownSpecialsSpec sets the spec returned for undefined specials.
func (c *LatexContextDb) SetUnknownSpecialsSpec(spec *SpecialsTextSpec) {
	c.db.UnknownSpecials = spec
}

// Categories returns the category names in lookup order.
func (c *LatexContextDb) Categories() []string { return c.db.Categories() }

// GetMacroSpec returns the text spec of a macro, or the unknown-macro spec (nil by default).
func (c *LatexContextDb) GetMacroSpec(name string) *MacroTextSpec { return c.db.Macro(name) }

// GetEnvironmentSpec returns the text spec of an environment, or the unknown-environment spec.
func (c *LatexContextDb) GetEnvironmentSpec(name string) *EnvironmentTextSpec {
	return c.db.Environment(name)
}

// GetSpecialsSpec returns the text spec of specials, or the unknown-specials spec.
func (c *LatexContextDb) GetSpecialsSpec(chars string) *SpecialsTextSpec {
	return c.db.SpecialsSpec(chars)
}

// IterMacroSpecs lists the macro specs of the given categories, or of all when nil.
func (c *LatexContextDb) IterMacroSpecs(categories []string) ([]*MacroTextSpec, error) {
	return c.db.MacroSpecs(categories)
}

// IterEnvironmentSpecs lists environment specs like IterMacroSpecs.
func (c *LatexContextDb) IterEnvironmentSpecs(categories []string) ([]*EnvironmentTextSpec, error) {
	return c.db.EnvironmentSpecs(categories)
}

// IterSpecialsSpecs lists specials specs like IterMacroSpecs.
func (c *LatexContextDb) IterSpecialsSpecs(categories []string) ([]*SpecialsTextSpec, error) {
	return c.db.SpecialsSpecs(categories)
}

// FilterContext is filter_context(); see latexwalker.LatexContextDb.FilterContext.
func (c *LatexContextDb) FilterContext(keepCategories, excludeCategories, keepWhich []string) *LatexContextDb {
	return &LatexContextDb{db: c.db.Filter(keepCategories, excludeCategories, keepWhich)}
}

type specCategory struct {
	name         string
	macros       []*MacroTextSpec
	environments []*EnvironmentTextSpec
	specials     []*SpecialsTextSpec
}

// GetDefaultLatexContextDb returns a new database holding pylatexenc's default text specs.
func GetDefaultLatexContextDb() *LatexContextDb {
	c := NewLatexContextDb()
	for _, cat := range defaultSpecs() {
		_ = c.AddContextCategory(cat.name, cat.macros, cat.environments, cat.specials, Placement{})
	}
	return c
}
