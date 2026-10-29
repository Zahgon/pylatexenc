package pyrt

import "sort"

const (
	hangulSBase  = 0xAC00
	hangulLBase  = 0x1100
	hangulVBase  = 0x1161
	hangulTBase  = 0x11A7
	hangulLCount = 19
	hangulVCount = 21
	hangulTCount = 28
	hangulNCount = hangulVCount * hangulTCount
	hangulSCount = hangulLCount * hangulNCount
)

// CombiningClass is unicodedata.combining(r).
func CombiningClass(r rune) int { return int(combiningClassMap[r]) }

func decompose(out []rune, r rune) []rune {
	if r >= hangulSBase && r < hangulSBase+hangulSCount {
		si := r - hangulSBase
		out = append(out, hangulLBase+si/hangulNCount, hangulVBase+(si%hangulNCount)/hangulTCount)
		if t := si % hangulTCount; t != 0 {
			out = append(out, hangulTBase+t)
		}
		return out
	}
	if d, ok := decompositionMap[r]; ok {
		return append(out, []rune(d)...)
	}
	return append(out, r)
}

func compose(a, b rune) (rune, bool) {
	if a >= hangulLBase && a < hangulLBase+hangulLCount && b >= hangulVBase && b < hangulVBase+hangulVCount {
		return hangulSBase + ((a-hangulLBase)*hangulVCount+(b-hangulVBase))*hangulTCount, true
	}
	if a >= hangulSBase && a < hangulSBase+hangulSCount && (a-hangulSBase)%hangulTCount == 0 &&
		b > hangulTBase && b < hangulTBase+hangulTCount {
		return a + (b - hangulTBase), true
	}
	c, ok := compositionMap[uint64(a)<<21|uint64(b)]
	return c, ok
}

// NFC is unicodedata.normalize('NFC', s).
func NFC(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	var d []rune
	for _, r := range s {
		d = decompose(d, r)
	}
	// Canonical ordering: stable sort of each run of non-starters by combining class.
	for i := 0; i < len(d); {
		if CombiningClass(d[i]) == 0 {
			i++
			continue
		}
		j := i
		for j < len(d) && CombiningClass(d[j]) != 0 {
			j++
		}
		run := d[i:j]
		sort.SliceStable(run, func(a, b int) bool { return CombiningClass(run[a]) < CombiningClass(run[b]) })
		i = j
	}
	// Canonical composition.
	out := make([]rune, 0, len(d))
	starter := -1
	lastClass := -1
	for _, r := range d {
		cc := CombiningClass(r)
		if starter >= 0 && (lastClass < cc || (lastClass == 0 && cc == 0 && len(out)-1 == starter)) {
			if c, ok := compose(out[starter], r); ok {
				out[starter] = c
				continue
			}
		}
		if cc == 0 {
			starter = len(out)
			lastClass = 0
		} else {
			lastClass = cc
		}
		out = append(out, r)
	}
	return string(out)
}
