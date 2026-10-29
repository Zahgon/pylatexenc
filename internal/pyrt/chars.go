// Package pyrt reproduces the pieces of CPython 3.12's runtime that pylatexenc's behaviour
// depends on: the str predicates and case mapping of its Unicode 15.0 database, NFC
// normalisation, repr(), the % operator on strings and textwrap.fill.
//
// Strings are handled as Python handles them, as sequences of code points: every length,
// index and column here counts runes, never bytes.
package pyrt

import (
	"sort"
	"strings"
	"unicode/utf8"
)

func inRanges(r rune, table []rune) bool {
	n := len(table) / 2
	i := sort.Search(n, func(i int) bool { return table[2*i+1] >= r })
	return i < n && table[2*i] <= r
}

// IsSpace reports whether str.isspace() holds for the one-character string r.
func IsSpace(r rune) bool { return inRanges(r, spaceRanges) }

// IsAlpha reports whether str.isalpha() holds for r.
func IsAlpha(r rune) bool { return inRanges(r, alphaRanges) }

// IsDecimal reports whether str.isdecimal() holds for r (the re module's \d).
func IsDecimal(r rune) bool { return inRanges(r, decimalRanges) }

// IsDigit reports whether str.isdigit() holds for r.
func IsDigit(r rune) bool { return inRanges(r, digitRanges) }

// IsNumeric reports whether str.isnumeric() holds for r.
func IsNumeric(r rune) bool { return inRanges(r, numericRanges) }

// IsWord reports whether the re module's \w matches r in a str pattern.
func IsWord(r rune) bool { return inRanges(r, wordRanges) }

// IsPrintable reports whether str.isprintable() holds for r.
func IsPrintable(r rune) bool { return inRanges(r, printableRanges) }

// Len is len() of a str: its number of code points.
func Len(s string) int { return utf8.RuneCountInString(s) }

// AllAlpha is str.isalpha() of a whole string: non-empty and every character alphabetic.
func AllAlpha(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !IsAlpha(r) {
			return false
		}
	}
	return true
}

// Strip is str.strip() with no argument.
func Strip(s string) string { return RStrip(LStrip(s)) }

// LStrip is str.lstrip() with no argument.
func LStrip(s string) string { return strings.TrimLeftFunc(s, IsSpace) }

// RStrip is str.rstrip() with no argument.
func RStrip(s string) string { return strings.TrimRightFunc(s, IsSpace) }

// Upper is str.upper(), with the full (one-to-many) case mapping.
func Upper(s string) string {
	var b strings.Builder
	for _, r := range s {
		if u, ok := upperMap[r]; ok {
			b.WriteString(u)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// RJust is str.rjust(width) with a space fill.
func RJust(s string, width int) string {
	n := Len(s)
	if n >= width {
		return s
	}
	return strings.Repeat(" ", width-n) + s
}

// Repr is repr() of a str.
func Repr(s string) string { return quote(s, false) }

// ASCII is ascii() of a str.
func ASCII(s string) string { return quote(s, true) }

func quote(s string, asciiOnly bool) string {
	q := '\''
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		q = '"'
	}
	var b strings.Builder
	b.WriteRune(q)
	const hex = "0123456789abcdef"
	for _, r := range s {
		switch {
		case r == q || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < ' ' || r == 0x7f:
			b.WriteString(`\x`)
			b.WriteByte(hex[r>>4])
			b.WriteByte(hex[r&0xf])
		case r < 0x7f:
			b.WriteRune(r)
		case !asciiOnly && IsPrintable(r):
			b.WriteRune(r)
		case r <= 0xff:
			b.WriteString(`\x`)
			b.WriteByte(hex[r>>4])
			b.WriteByte(hex[r&0xf])
		case r <= 0xffff:
			b.WriteString(`\u`)
			for shift := 12; shift >= 0; shift -= 4 {
				b.WriteByte(hex[(r>>uint(shift))&0xf])
			}
		default:
			b.WriteString(`\U`)
			for shift := 28; shift >= 0; shift -= 4 {
				b.WriteByte(hex[(r>>uint(shift))&0xf])
			}
		}
	}
	b.WriteRune(q)
	return b.String()
}

// ExpandTabs is str.expandtabs(tabsize).
func ExpandTabs(s string, tabsize int) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			if tabsize > 0 {
				n := tabsize - col%tabsize
				b.WriteString(strings.Repeat(" ", n))
				col += n
			}
			continue
		}
		b.WriteRune(r)
		col++
		if r == '\n' || r == '\r' {
			col = 0
		}
	}
	return b.String()
}
