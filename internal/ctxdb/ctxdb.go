// Package ctxdb is the category database shared by latexwalker's and latex2text's context
// databases: specs are grouped in named categories, looked up in category order, and
// within a category a later spec with the same key replaces an earlier one in place.
package ctxdb

import (
	"fmt"
	"strings"

	"github.com/phfaist/pylatexenc"
	"github.com/phfaist/pylatexenc/internal/pyrt"
)

// OrderedMap is a Python dict keyed by string.
type OrderedMap[V any] struct {
	keys []string
	m    map[string]V
}

// NewOrderedMap builds a dict from items in order, the last item of a key winning while the
// key keeps the position of its first occurrence.
func NewOrderedMap[V any](items []V, key func(V) string) *OrderedMap[V] {
	om := &OrderedMap[V]{m: map[string]V{}}
	for _, it := range items {
		k := key(it)
		if _, ok := om.m[k]; !ok {
			om.keys = append(om.keys, k)
		}
		om.m[k] = it
	}
	return om
}

// Get returns the value of key.
func (om *OrderedMap[V]) Get(k string) (V, bool) {
	v, ok := om.m[k]
	return v, ok
}

// Keys returns the keys in order.
func (om *OrderedMap[V]) Keys() []string { return om.keys }

// Values returns the values in key order.
func (om *OrderedMap[V]) Values() []V {
	out := make([]V, len(om.keys))
	for i, k := range om.keys {
		out[i] = om.m[k]
	}
	return out
}

// Category holds one category's specs.
type Category[M, E, S any] struct {
	Macros       *OrderedMap[M]
	Environments *OrderedMap[E]
	Specials     *OrderedMap[S]
}

// Keys extracts the lookup key of each kind of spec.
type Keys[M, E, S any] struct {
	Macro       func(M) string
	Environment func(E) string
	Specials    func(S) string
}

// Db is LatexContextDb.
type Db[M, E, S any] struct {
	keys       Keys[M, E, S]
	categories []string
	d          map[string]*Category[M, E, S]

	UnknownMacro       M
	UnknownEnvironment E
	UnknownSpecials    S
}

// New returns an empty database.
func New[M, E, S any](keys Keys[M, E, S]) *Db[M, E, S] {
	return &Db[M, E, S]{keys: keys, d: map[string]*Category[M, E, S]{}}
}

// Placement says where AddCategory puts a new category: first (Prepend), before an existing
// category (InsertBefore, or first when it is not registered), after one (InsertAfter, or
// last when it is not registered), or last when nothing is set.
type Placement struct {
	Prepend      bool
	InsertBefore string
	InsertAfter  string
}

// AddCategory is add_context_category().
func (db *Db[M, E, S]) AddCategory(category string, macros []M, envs []E, specials []S, at Placement) error {
	for _, c := range db.categories {
		if c == category {
			return &pylatexenc.BuiltinError{Class: "ValueError",
				Message: fmt.Sprintf("Category %s is already registered in the context database", category)}
		}
	}
	set := 0
	for _, b := range []bool{at.Prepend, at.InsertBefore != "", at.InsertAfter != ""} {
		if b {
			set++
		}
	}
	if set > 1 {
		return &pylatexenc.BuiltinError{Class: "TypeError",
			Message: "add_context_category(): You may only specify one of prepend=True, insert_before=... or insert_after=..."}
	}
	index := func(name string) int {
		for i, c := range db.categories {
			if c == name {
				return i
			}
		}
		return -1
	}
	i := len(db.categories)
	switch {
	case at.Prepend:
		i = 0
	case at.InsertBefore != "":
		i = max(index(at.InsertBefore), 0)
	case at.InsertAfter != "":
		if j := index(at.InsertAfter); j >= 0 {
			i = j + 1
		}
	}
	db.categories = append(db.categories, "")
	copy(db.categories[i+1:], db.categories[i:])
	db.categories[i] = category
	db.d[category] = &Category[M, E, S]{
		Macros:       NewOrderedMap(macros, db.keys.Macro),
		Environments: NewOrderedMap(envs, db.keys.Environment),
		Specials:     NewOrderedMap(specials, db.keys.Specials),
	}
	return nil
}

// Categories returns the category names in lookup order.
func (db *Db[M, E, S]) Categories() []string { return append([]string(nil), db.categories...) }

// Category returns the specs of a registered category.
func (db *Db[M, E, S]) Category(name string) *Category[M, E, S] { return db.d[name] }

// Macro looks a macro up, falling back to the unknown-macro spec.
func (db *Db[M, E, S]) Macro(name string) M {
	for _, c := range db.categories {
		if v, ok := db.d[c].Macros.Get(name); ok {
			return v
		}
	}
	return db.UnknownMacro
}

// Environment looks an environment up, falling back to the unknown-environment spec.
func (db *Db[M, E, S]) Environment(name string) E {
	for _, c := range db.categories {
		if v, ok := db.d[c].Environments.Get(name); ok {
			return v
		}
	}
	return db.UnknownEnvironment
}

// SpecialsSpec looks specials up by their exact characters.
func (db *Db[M, E, S]) SpecialsSpec(chars string) S {
	for _, c := range db.categories {
		if v, ok := db.d[c].Specials.Get(chars); ok {
			return v
		}
	}
	return db.UnknownSpecials
}

// TestForSpecials returns the specials whose characters start s at pos, the longest match
// winning and the first found winning among equally long ones.
func (db *Db[M, E, S]) TestForSpecials(s []rune, pos int) (S, bool) {
	var best S
	bestLen := 0
	found := false
	for _, c := range db.categories {
		cat := db.d[c]
		for _, k := range cat.Specials.Keys() {
			kr := []rune(k)
			if len(kr) > bestLen && hasPrefixAt(s, kr, pos) {
				best, _ = cat.Specials.Get(k)
				bestLen = len(kr)
				found = true
			}
		}
	}
	return best, found
}

func hasPrefixAt(s, prefix []rune, pos int) bool {
	if pos < 0 || pos > len(s) || len(s)-pos < len(prefix) {
		return false
	}
	for i, r := range prefix {
		if s[pos+i] != r {
			return false
		}
	}
	return true
}

func (db *Db[M, E, S]) checkCategories(categories []string, what string) ([]string, error) {
	if categories == nil {
		return db.categories, nil
	}
	for _, c := range categories {
		found := false
		for _, have := range db.categories {
			if have == c {
				found = true
				break
			}
		}
		if !found {
			return nil, &pylatexenc.BuiltinError{Class: "ValueError", Message: fmt.Sprintf(
				"Invalid latex %s spec db category: %s (Expected one of %s)",
				what, pyrt.Repr(c), ReprList(db.categories))}
		}
	}
	return categories, nil
}

// MacroSpecs lists the macro specs of the given categories (all when nil).
func (db *Db[M, E, S]) MacroSpecs(categories []string) ([]M, error) {
	cats, err := db.checkCategories(categories, "macro")
	if err != nil {
		return nil, err
	}
	out := []M{}
	for _, c := range cats {
		out = append(out, db.d[c].Macros.Values()...)
	}
	return out, nil
}

// EnvironmentSpecs lists the environment specs of the given categories (all when nil).
func (db *Db[M, E, S]) EnvironmentSpecs(categories []string) ([]E, error) {
	cats, err := db.checkCategories(categories, "environment")
	if err != nil {
		return nil, err
	}
	out := []E{}
	for _, c := range cats {
		out = append(out, db.d[c].Environments.Values()...)
	}
	return out, nil
}

// SpecialsSpecs lists the specials specs of the given categories (all when nil). Its error
// message names environments, as pylatexenc's does.
func (db *Db[M, E, S]) SpecialsSpecs(categories []string) ([]S, error) {
	cats, err := db.checkCategories(categories, "environment")
	if err != nil {
		return nil, err
	}
	out := []S{}
	for _, c := range cats {
		out = append(out, db.d[c].Specials.Values()...)
	}
	return out, nil
}

// Filter is filter_context().
func (db *Db[M, E, S]) Filter(keepCategories, excludeCategories, keepWhich []string) *Db[M, E, S] {
	nd := New(db.keys)
	nd.UnknownMacro = db.UnknownMacro
	nd.UnknownEnvironment = db.UnknownEnvironment
	nd.UnknownSpecials = db.UnknownSpecials
	keepMacros := len(keepWhich) == 0 || contains(keepWhich, "macros")
	keepEnvs := len(keepWhich) == 0 || contains(keepWhich, "environments")
	keepSpecials := len(keepWhich) == 0 || contains(keepWhich, "specials")
	for _, c := range db.categories {
		if len(keepCategories) > 0 && !contains(keepCategories, c) {
			continue
		}
		if len(excludeCategories) > 0 && contains(excludeCategories, c) {
			continue
		}
		cat := db.d[c]
		var ms []M
		var es []E
		var ss []S
		if keepMacros {
			ms = cat.Macros.Values()
		}
		if keepEnvs {
			es = cat.Environments.Values()
		}
		if keepSpecials {
			ss = cat.Specials.Values()
		}
		_ = nd.AddCategory(c, ms, es, ss, Placement{})
	}
	return nd
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ReprList is repr() of a list of str.
func ReprList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = pyrt.Repr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
