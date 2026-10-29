// Package pylatexenc is a Go port of pylatexenc 2.11: a LaTeX parser (latexwalker), a
// LaTeX-to-text converter (latex2text) and a Unicode-to-LaTeX encoder (latexencode).
package pylatexenc

// Version is the pylatexenc release this port reproduces.
const Version = "2.11"

// BuiltinError is an error that pylatexenc 2.11 raises as one of Python's built-in
// exceptions. Class is the exception class name ("ValueError", "TypeError", "KeyError",
// "IndexError" or "AttributeError") and Message is str() of the exception.
type BuiltinError struct {
	Class   string
	Message string
}

func (e *BuiltinError) Error() string { return e.Class + ": " + e.Message }
