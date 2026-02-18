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
	if len(raw) == 0 {
		return plainSpans(text)
	}

	sort.Slice(raw, func(i, j int) bool {
		if raw[i].Start == raw[j].Start {
			return raw[i].End < raw[j].End
		}
		return raw[i].Start < raw[j].Start
	})

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
	if len(spans) == 0 {
		return []Span{{Start: 0, End: runeLen, Cat: TokenPlain}}
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

	if len(clean) == 0 {
		return []Span{{Start: 0, End: runeLen, Cat: TokenPlain}}
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
			out = append(out, Span{Start: cursor, End: start, Cat: TokenPlain})
		}

		if len(out) > 0 {
			last := &out[len(out)-1]
			if last.End == start && last.Cat == span.Cat {
				last.End = end
			} else {
				out = append(out, Span{Start: start, End: end, Cat: span.Cat})
			}
		} else {
			out = append(out, Span{Start: start, End: end, Cat: span.Cat})
		}

		cursor = end
	}

	if cursor < runeLen {
		out = append(out, Span{Start: cursor, End: runeLen, Cat: TokenPlain})
	}

	if len(out) == 0 {
		return []Span{{Start: 0, End: runeLen, Cat: TokenPlain}}
	}

	return out
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
