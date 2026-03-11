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

func TestHighlighterDetectLanguageRecognizesZigFiles(t *testing.T) {
	lang := highlighter.DetectLanguage("build.zig")
	if lang != highlighter.LangZig {
		t.Fatalf("lang = %q, want %q", lang, highlighter.LangZig)
	}
}

func TestHighlighterDetectLanguageRecognizesAdditionalFiles(t *testing.T) {
	tests := []struct {
		path string
		want highlighter.LangID
	}{
		{path: "Program.cs", want: highlighter.LangCSharp},
		{path: "Main.java", want: highlighter.LangJava},
		{path: "Main.kt", want: highlighter.LangKotlin},
		{path: "index.php", want: highlighter.LangPHP},
		{path: "lib/search.rb", want: highlighter.LangRuby},
		{path: "Gemfile", want: highlighter.LangRuby},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			lang := highlighter.DetectLanguage(tt.path)
			if lang != tt.want {
				t.Fatalf("lang = %q, want %q", lang, tt.want)
			}
		})
	}
}

func TestHighlighterDetectLanguageWithShebangRecognizesRuby(t *testing.T) {
	lang := highlighter.DetectLanguageWithShebang("script", "#!/usr/bin/env ruby")
	if lang != highlighter.LangRuby {
		t.Fatalf("lang = %q, want %q", lang, highlighter.LangRuby)
	}
}
