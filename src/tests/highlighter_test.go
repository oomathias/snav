package main_test

import (
	"testing"

	"snav/internal/highlighter"
)

func TestHighlighterParseHighlightContextMode(t *testing.T) {
	mode, err := highlighter.ParseHighlightContextMode("file")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if mode != highlighter.HighlightContextFile {
		t.Fatalf("mode = %q, want %q", mode, highlighter.HighlightContextFile)
	}
}

func TestHighlighterDetectLanguageWithShebang(t *testing.T) {
	lang := highlighter.DetectLanguageWithShebang("script", "#!/usr/bin/env python")
	if lang != highlighter.LangPython {
		t.Fatalf("lang = %q, want %q", lang, highlighter.LangPython)
	}
}
