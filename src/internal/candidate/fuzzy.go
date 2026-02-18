package candidate

import (
	"strings"
	"unicode"
)

func fuzzyScore(text string, queryRaw []rune, queryLower []rune, caseSensitive bool) (int, bool) {
	if len(queryLower) == 0 {
		return 0, true
	}

	qi := 0
	last := -2
	score := 0
	runeIdx := 0
	var prev rune
	hasPrev := false
	caseMatches := 0

	for _, raw := range text {
		r := lowerRuneFast(raw)

		if qi < len(queryLower) && r == queryLower[qi] {
			bonus := 10
			if runeIdx == 0 || (hasPrev && isBoundaryRune(prev)) {
				bonus += 8
			}
			if last+1 == runeIdx {
				bonus += 6
			}
			if caseSensitive && raw == queryRaw[qi] {
				bonus += 4
				caseMatches++
			}

			score += bonus
			last = runeIdx
			qi++
		}

		prev = r
		hasPrev = true
		runeIdx++
	}

	if qi != len(queryLower) {
		return 0, false
	}

	if runeIdx > len(queryLower) {
		score -= runeIdx - len(queryLower)
	}
	if runeIdx < 40 {
		score += 40 - runeIdx
	}
	if caseMatches > 0 {
		score += caseMatches * 3
	}

	return score, true
}

func FuzzyPositionsRunes(text string, queryLower []rune) []int {
	return fuzzyPositionsRunes(text, queryLower)
}

func fuzzyPositionsRunes(text string, queryLower []rune) []int {
	if len(queryLower) == 0 {
		return nil
	}

	out := make([]int, 0, len(queryLower))
	qi := 0
	idx := 0
	for _, raw := range text {
		if qi >= len(queryLower) {
			break
		}
		if lowerRuneFast(raw) == queryLower[qi] {
			out = append(out, idx)
			qi++
		}
		idx++
	}
	if qi != len(queryLower) {
		return nil
	}
	return out
}

func TrimRunes(s string) []rune {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return []rune(s)
}

func LowerRunes(r []rune) []rune {
	if len(r) == 0 {
		return nil
	}
	out := make([]rune, len(r))
	for i := range r {
		out[i] = lowerRuneFast(r[i])
	}
	return out
}

func lowerRuneFast(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	if r <= unicode.MaxASCII {
		return r
	}
	return unicode.ToLower(r)
}

func isBoundaryRune(r rune) bool {
	switch r {
	case '_', '-', '/', '.', ':':
		return true
	}
	if r <= unicode.MaxASCII {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
