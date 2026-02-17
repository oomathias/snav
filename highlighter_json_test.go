package main

import (
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func TestJSONTreeSitterHighlighting(t *testing.T) {
	h := NewHighlighter(HighlighterConfig{
		CacheSize:   64,
		Workers:     1,
		DefaultMode: HighlightContextSynthetic,
	})

	parser := sitter.NewParser()
	req := HighlightRequest{
		Lang: LangJSON,
		Text: `"count": 42,`,
		Mode: HighlightContextSynthetic,
	}

	spans := h.highlightWithParser(parser, req)
	if len(spans) == 0 {
		t.Fatalf("expected spans for JSON line")
	}

	hasNonPlain := false
	for _, span := range spans {
		if span.Cat != TokenPlain {
			hasNonPlain = true
			break
		}
	}

	if !hasNonPlain {
		t.Fatalf("expected non-plain token categories for JSON line")
	}
}
