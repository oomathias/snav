package highlighter

import (
	"slices"
	"strings"
)

func buildSliceSource(lines []string, targetIndex int) ([]byte, int, int, bool) {
	if targetIndex < 0 || targetIndex >= len(lines) {
		return nil, 0, 0, false
	}

	var sb strings.Builder
	lineStart := 0
	lineEnd := 0

	for i := range lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		if i == targetIndex {
			lineStart = sb.Len()
		}
		sb.WriteString(lines[i])
		if i == targetIndex {
			lineEnd = sb.Len()
		}
	}

	return []byte(sb.String()), lineStart, lineEnd, true
}

func projectSpansToDisplay(baseSpans []Span, sourceLine string, displayLine string) ([]Span, bool) {
	displayRunes := []rune(displayLine)

	normalizedSource, normalizedToSource := normalizeLineForDisplayRunes(sourceLine)

	prefixLen := len(displayRunes)
	if prefixLen >= 3 && strings.HasSuffix(displayLine, "...") {
		prefixLen -= 3
	}

	if prefixLen > len(normalizedSource) {
		return nil, false
	}
	if !slices.Equal(displayRunes[:prefixLen], normalizedSource[:prefixLen]) {
		return nil, false
	}

	projected := make([]Span, 0, len(baseSpans)+2)

	spanIdx := 0
	for i := 0; i < prefixLen; i++ {
		srcIdx := normalizedToSource[i]
		cat := TokenPlain
		for spanIdx < len(baseSpans) && srcIdx >= baseSpans[spanIdx].End {
			spanIdx++
		}
		if spanIdx < len(baseSpans) {
			span := baseSpans[spanIdx]
			if srcIdx >= span.Start && srcIdx < span.End {
				cat = span.Cat
			}
		}
		projected = appendMergedSpan(projected, i, i+1, cat)
	}

	if prefixLen < len(displayRunes) {
		projected = appendMergedSpan(projected, prefixLen, len(displayRunes), TokenPlain)
	}

	return normalizeSpans(projected, len(displayRunes)), true
}

func normalizeLineForDisplayRunes(line string) ([]rune, []int) {
	source := []rune(line)
	out := make([]rune, 0, len(source))
	indexMap := make([]int, 0, len(source))

	for i, r := range source {
		switch r {
		case '\r':
			continue
		case '\n':
			out = append(out, ' ')
			indexMap = append(indexMap, i)
		case '\t':
			for j := 0; j < 4; j++ {
				out = append(out, ' ')
				indexMap = append(indexMap, i)
			}
		default:
			out = append(out, r)
			indexMap = append(indexMap, i)
		}
	}

	return out, indexMap
}
