package latexwalker

import (
	"fmt"
	"strings"

	"github.com/phfaist/pylatexenc/internal/pyrt"
)

// Error is LatexWalkerError, a generic error of the walker.
type Error struct {
	Msg string
}

func (e *Error) Error() string { return e.Msg }

// OpenContext is one LaTeX block that was open when a parse error happened.
type OpenContext struct {
	What   string
	Pos    int
	Lineno int
	Colno  int
}

// ParseError is LatexWalkerParseError. Lineno is 0 when the error carries no line
// information; its position is then shown as "@ char POS".
type ParseError struct {
	Msg          string
	S            string
	Pos          int
	Lineno       int
	Colno        int
	OpenContexts []OpenContext
	InputSource  string
}

func fmtPos(pos, lineno, colno int) string {
	if lineno != 0 {
		return fmt.Sprintf("@(%d,%d)", lineno, colno)
	}
	return fmt.Sprintf("@ char %d", pos)
}

// Error is str() of the Python exception.
func (e *ParseError) Error() string {
	msg := e.Msg
	if e.InputSource != "" {
		msg += "  in " + e.InputSource
	}
	var b strings.Builder
	b.WriteString(msg + " " + fmtPos(e.Pos, e.Lineno, e.Colno))
	if len(e.OpenContexts) > 0 {
		b.WriteString("\nOpen LaTeX blocks:\n")
		for i := len(e.OpenContexts) - 1; i >= 0; i-- {
			c := e.OpenContexts[i]
			b.WriteString("        " + pyrt.RJust(fmtPos(c.Pos, c.Lineno, c.Colno), 10) + "  " + c.What + "\n")
		}
	}
	return b.String()
}

// EndOfStreamError is LatexWalkerEndOfStream: the input ended where a token was expected.
// FinalSpace is the whitespace read before the end.
type EndOfStreamError struct {
	FinalSpace string
}

func (e *EndOfStreamError) Error() string { return "" }
