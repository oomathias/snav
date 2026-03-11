package highlighter

import (
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func TestZigTreeSitterHighlighting(t *testing.T) {
	h := NewHighlighter(HighlighterConfig{
		CacheSize:   64,
		Workers:     1,
		DefaultMode: HighlightContextSynthetic,
	})

	parser := sitter.NewParser()
	req := HighlightRequest{
		Lang: LangZig,
		Text: "pub fn main() !void {}",
		Mode: HighlightContextSynthetic,
	}

	spans := h.HighlightWithParser(parser, req)
	if len(spans) == 0 {
		t.Fatalf("expected spans for Zig line")
	}

	hasNonPlain := false
	for _, span := range spans {
		if span.Cat != TokenPlain {
			hasNonPlain = true
			break
		}
	}

	if !hasNonPlain {
		t.Fatalf("expected non-plain token categories for Zig line")
	}
}
