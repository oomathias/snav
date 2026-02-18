package highlighter

import "strings"

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
	if displayLine == "" {
		return nil, true
	}

	displayRunes := []rune(displayLine)
	if len(displayRunes) == 0 {
		return nil, true
	}

	normalizedSource, normalizedToSource := normalizeLineForDisplayRunes(sourceLine)

	prefixLen := len(displayRunes)
	hasEllipsis := false
	if len(displayRunes) >= 3 && strings.HasSuffix(displayLine, "...") {
		hasEllipsis = true
		prefixLen = len(displayRunes) - 3
	}

	if prefixLen > len(normalizedSource) {
		return nil, false
	}
	if !runesEqual(displayRunes[:prefixLen], normalizedSource[:prefixLen]) {
		return nil, false
	}

	projected := make([]Span, 0, len(baseSpans)+2)
	appendSpan := func(start int, end int, cat TokenCategory) {
		if end <= start {
			return
		}
		if len(projected) > 0 {
			last := &projected[len(projected)-1]
			if last.End == start && last.Cat == cat {
				last.End = end
				return
			}
		}
		projected = append(projected, Span{Start: start, End: end, Cat: cat})
	}

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
		appendSpan(i, i+1, cat)
	}

	if hasEllipsis {
		appendSpan(prefixLen, len(displayRunes), TokenPlain)
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

func runesEqual(a []rune, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
