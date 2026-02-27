package highlighter

import (
	"sort"
	"unicode/utf8"
)

func scaffoldLine(lang LangID, line string) ([]byte, int, int) {
	prefix := ""
	suffix := "\n"

	switch lang {
	case LangGo:
		prefix = "package p\nfunc _snav_() {\n"
		suffix = "\n}\n"
	case LangRust:
		prefix = "fn _snav_() {\n"
		suffix = "\n}\n"
	case LangJavaScript, LangTypeScript, LangTSX:
		prefix = "function _snav_() {\n"
		suffix = "\n}\n"
	case LangC, LangCPP:
		prefix = "void _snav_() {\n"
		suffix = "\n}\n"
	case LangJSON:
		prefix = "{\n"
		suffix = "\n}\n"
	}

	source := []byte(prefix + line + suffix)
	start := len(prefix)
	end := start + len(line)
	return source, start, end
}

func plainSpans(text string) []Span {
	runeLen := utf8.RuneCountInString(text)
	if runeLen == 0 {
		return nil
	}
	return []Span{{Start: 0, End: runeLen, Cat: TokenPlain}}
}

func buildMergedSpans(text string, raw []rawSpan) []Span {
	runeLen := utf8.RuneCountInString(text)
	if runeLen == 0 {
		return nil
	}

	spans := make([]Span, 0, len(raw)+2)
	for _, rs := range raw {
		startRune := byteToRuneIndex(text, rs.Start)
		endRune := byteToRuneIndex(text, rs.End)
		if endRune <= startRune {
			continue
		}
		spans = append(spans, Span{Start: startRune, End: endRune, Cat: rs.Cat})
	}

	return normalizeSpans(spans, runeLen)
}

func normalizeSpans(spans []Span, runeLen int) []Span {
	if runeLen <= 0 {
		return nil
	}

	clean := make([]Span, 0, len(spans))
	for _, span := range spans {
		start := span.Start
		end := span.End
		if start < 0 {
			start = 0
		}
		if end > runeLen {
			end = runeLen
		}
		if end <= start {
			continue
		}
		clean = append(clean, Span{Start: start, End: end, Cat: span.Cat})
	}

	sort.Slice(clean, func(i, j int) bool {
		if clean[i].Start == clean[j].Start {
			return clean[i].End < clean[j].End
		}
		return clean[i].Start < clean[j].Start
	})

	out := make([]Span, 0, len(clean)+2)
	cursor := 0
	for _, span := range clean {
		start := span.Start
		end := span.End

		if start < cursor {
			start = cursor
		}
		if end <= start {
			continue
		}

		if start > cursor {
			out = appendMergedSpan(out, cursor, start, TokenPlain)
		}
		out = appendMergedSpan(out, start, end, span.Cat)

		cursor = end
	}

	if cursor < runeLen {
		out = appendMergedSpan(out, cursor, runeLen, TokenPlain)
	}

	return out
}

func appendMergedSpan(spans []Span, start int, end int, cat TokenCategory) []Span {
	if end <= start {
		return spans
	}

	if len(spans) > 0 {
		last := &spans[len(spans)-1]
		if last.End == start && last.Cat == cat {
			last.End = end
			return spans
		}
	}

	return append(spans, Span{Start: start, End: end, Cat: cat})
}

func byteToRuneIndex(s string, b int) int {
	if b <= 0 {
		return 0
	}
	if b >= len(s) {
		return utf8.RuneCountInString(s)
	}
	return utf8.RuneCountInString(s[:b])
}
