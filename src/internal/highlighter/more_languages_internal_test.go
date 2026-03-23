package highlighter

import (
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func TestAdditionalLanguageTreeSitterHighlighting(t *testing.T) {
	h := NewHighlighter(HighlighterConfig{
		CacheSize:   64,
		Workers:     1,
		DefaultMode: HighlightContextSynthetic,
	})

	parser := sitter.NewParser()
	tests := []struct {
		name string
		lang LangID
		text string
	}{
		{name: "csharp", lang: LangCSharp, text: "public class SearchIndex {}"},
		{name: "java", lang: LangJava, text: "public class SearchIndex {}"},
		{name: "kotlin", lang: LangKotlin, text: "data class SearchIndex(val id: Int)"},
		{name: "php", lang: LangPHP, text: "function search_index() {}"},
		{name: "ruby", lang: LangRuby, text: "def search_index; end"},
		{name: "swift", lang: LangSwift, text: "final class ServiceManager {}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spans := h.HighlightWithParser(parser, HighlightRequest{
				Lang: tt.lang,
				Text: tt.text,
				Mode: HighlightContextSynthetic,
			})
			if len(spans) == 0 {
				t.Fatalf("expected spans for %s line", tt.name)
			}

			hasNonPlain := false
			for _, span := range spans {
				if span.Cat != TokenPlain {
					hasNonPlain = true
					break
				}
			}

			if !hasNonPlain {
				t.Fatalf("expected non-plain token categories for %s line", tt.name)
			}
		})
	}
}
