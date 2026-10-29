package pyrt

import (
	"strings"
)

func isWrapSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\x0b', '\x0c', '\r', ' ':
		return true
	}
	return false
}

// isLetter is textwrap's letter class, [^\d\W].
func isLetter(r rune) bool { return IsWord(r) && !IsDecimal(r) }

// isWordPunct is textwrap's word_punct class, [\w!"'&.,?].
func isWordPunct(r rune) bool {
	switch r {
	case '!', '"', '\'', '&', '.', ',', '?':
		return true
	}
	return IsWord(r)
}

// wordsepMatch tries TextWrapper.wordsep_re at position p of t and returns the end of the
// match.
func wordsepMatch(t []rune, p int) (int, bool) {
	n := len(t)
	at := func(i int) rune {
		if i >= 0 && i < n {
			return t[i]
		}
		return -1
	}
	letter := func(i int) bool { return i >= 0 && i < n && isLetter(t[i]) }
	wordPunct := func(i int) bool { return i >= 0 && i < n && isWordPunct(t[i]) }
	word := func(i int) bool { return i >= 0 && i < n && IsWord(t[i]) }
	// dashesThenWord reports whether t[i:] starts with two or more '-' followed by a \w.
	dashesThenWord := func(i int) (int, bool) {
		j := i
		for j < n && t[j] == '-' {
			j++
		}
		if j-i >= 2 && word(j) {
			return j, true
		}
		return 0, false
	}
	if p >= n {
		return 0, false
	}
	if isWrapSpace(t[p]) {
		q := p
		for q < n && isWrapSpace(t[q]) {
			q++
		}
		return q, true
	}
	if wordPunct(p - 1) {
		if end, ok := dashesThenWord(p); ok {
			return end, true
		}
	}
	for q := p + 1; ; q++ {
		if isWrapSpace(t[q-1]) {
			return 0, false
		}
		if at(q) == '-' && ((letter(q-2) && letter(q-1)) || (letter(q-3) && at(q-2) == '-' && letter(q-1))) &&
			letter(q+1) && (letter(q+2) || (at(q+2) == '-' && letter(q+3))) {
			return q + 1, true
		}
		if q == n || isWrapSpace(t[q]) {
			return q, true
		}
		if wordPunct(q - 1) {
			if _, ok := dashesThenWord(q); ok {
				return q, true
			}
		}
	}
}

// wrapSplit is TextWrapper._split with break_on_hyphens: wordsep_re.split(text) with the
// empty strings dropped.
func wrapSplit(text string) [][]rune {
	t := []rune(text)
	var chunks [][]rune
	last := 0
	for p := 0; p < len(t); {
		end, ok := wordsepMatch(t, p)
		if !ok {
			p++
			continue
		}
		if p > last {
			chunks = append(chunks, t[last:p])
		}
		chunks = append(chunks, t[p:end])
		last = end
		p = end
	}
	if last < len(t) {
		chunks = append(chunks, t[last:])
	}
	return chunks
}

func blank(chunk []rune) bool { return strings.TrimFunc(string(chunk), IsSpace) == "" }

// Fill is textwrap.fill(text, width, initial_indent=initialIndent) with every other option at
// its default.
func Fill(text string, width int, initialIndent string) (string, error) {
	if width <= 0 {
		return "", valueError("invalid width " + itoa(width) + " (must be > 0)")
	}
	text = ExpandTabs(text, 8)
	text = strings.Map(func(r rune) rune {
		if isWrapSpace(r) {
			return ' '
		}
		return r
	}, text)
	chunks := wrapSplit(text)
	// Reverse so that the next chunk is at the end, as in _wrap_chunks.
	for i, j := 0, len(chunks)-1; i < j; i, j = i+1, j-1 {
		chunks[i], chunks[j] = chunks[j], chunks[i]
	}
	var lines []string
	for len(chunks) > 0 {
		var curLine [][]rune
		curLen := 0
		indent := ""
		if len(lines) == 0 {
			indent = initialIndent
		}
		w := width - Len(indent)
		if blank(chunks[len(chunks)-1]) && len(lines) > 0 {
			chunks = chunks[:len(chunks)-1]
		}
		for len(chunks) > 0 {
			l := len(chunks[len(chunks)-1])
			if curLen+l <= w {
				curLine = append(curLine, chunks[len(chunks)-1])
				chunks = chunks[:len(chunks)-1]
				curLen += l
			} else {
				break
			}
		}
		if len(chunks) > 0 && len(chunks[len(chunks)-1]) > w {
			spaceLeft := 1
			if w >= 1 {
				spaceLeft = w - curLen
			}
			end := spaceLeft
			chunk := chunks[len(chunks)-1]
			if len(chunk) > spaceLeft {
				hyphen := -1
				for i := min(spaceLeft, len(chunk)) - 1; i >= 0; i-- {
					if chunk[i] == '-' {
						hyphen = i
						break
					}
				}
				if hyphen > 0 {
					for _, c := range chunk[:hyphen] {
						if c != '-' {
							end = hyphen + 1
							break
						}
					}
				}
			}
			if end > len(chunk) {
				end = len(chunk)
			}
			curLine = append(curLine, chunk[:end])
			chunks[len(chunks)-1] = chunk[end:]
			curLen = 0
			for _, c := range curLine {
				curLen += len(c)
			}
		}
		if len(curLine) > 0 && blank(curLine[len(curLine)-1]) {
			curLen -= len(curLine[len(curLine)-1])
			curLine = curLine[:len(curLine)-1]
		}
		if len(curLine) > 0 {
			var b strings.Builder
			b.WriteString(indent)
			for _, c := range curLine {
				b.WriteString(string(c))
			}
			lines = append(lines, b.String())
		}
	}
	return strings.Join(lines, "\n"), nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
