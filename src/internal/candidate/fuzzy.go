package candidate

import (
	"strings"
	"unicode"
)

func fuzzyScore(text string, queryRaw []rune, queryLower []rune, caseSensitive bool) (int, int, bool) {
	if len(queryLower) == 0 {
		return 0, 0, true
	}

	queryLen := nonSpaceRuneCount(queryLower)
	if queryLen == 0 {
		return 0, 0, true
	}

	qi := skipLeadingSpaces(queryLower, 0)
	if qi == len(queryLower) {
		return 0, 0, true
	}

	last := -2
	first := -1
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
			if caseSensitive && qi < len(queryRaw) && raw == queryRaw[qi] {
				bonus += 4
				caseMatches++
			}

			score += bonus
			if first < 0 {
				first = runeIdx
			}
			last = runeIdx
			qi++
			qi = skipLeadingSpaces(queryLower, qi)
		}

		prev = r
		hasPrev = true
		runeIdx++
	}

	qi = skipLeadingSpaces(queryLower, qi)
	if qi != len(queryLower) {
		return 0, 0, false
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

	span := 0
	if first >= 0 {
		span = last - first + 1
	}
	if span > 0 {
		if span == queryLen {
			score += 12
		} else if span > queryLen {
			score -= (span - queryLen) * 2
		}
	}

	return score, span, true
}

func FuzzyPositionsRunes(text string, queryLower []rune) []int {
	if len(queryLower) == 0 {
		return nil
	}

	out := make([]int, 0, len(queryLower))
	qi := skipLeadingSpaces(queryLower, 0)
	if qi == len(queryLower) {
		return nil
	}

	idx := 0
	for _, raw := range text {
		if qi >= len(queryLower) {
			break
		}
		if lowerRuneFast(raw) == queryLower[qi] {
			out = append(out, idx)
			qi++
			qi = skipLeadingSpaces(queryLower, qi)
		}
		idx++
	}
	qi = skipLeadingSpaces(queryLower, qi)
	if qi != len(queryLower) {
		return nil
	}
	return out
}

func skipLeadingSpaces(r []rune, i int) int {
	for i < len(r) && unicode.IsSpace(r[i]) {
		i++
	}
	return i
}

func nonSpaceRuneCount(r []rune) int {
	n := 0
	for _, v := range r {
		if !unicode.IsSpace(v) {
			n++
		}
	}
	return n
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
