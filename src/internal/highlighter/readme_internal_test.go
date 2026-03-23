package highlighter

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestReadmeSupportedLanguagesListMatchesHighlighter(t *testing.T) {
	readmePath := filepath.Join("..", "..", "..", "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", readmePath, err)
	}

	got := readmeSupportedLanguages(t, string(data))
	want := implementedSupportedLanguages(t)

	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("README supported languages = %#v, want %#v", got, want)
	}
}

func readmeSupportedLanguages(t *testing.T, text string) []string {
	t.Helper()

	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	const marker = "Syntax-highlighted preview currently supports:"

	var out []string
	collecting := false
	for _, line := range lines {
		switch {
		case strings.TrimSpace(line) == marker:
			collecting = true
		case collecting && strings.HasPrefix(line, "- "):
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "- ")))
		case collecting && len(out) > 0:
			return out
		}
	}

	t.Fatalf("could not find supported languages section in README")
	return nil
}

func implementedSupportedLanguages(t *testing.T) []string {
	t.Helper()

	displayNames := map[LangID]string{
		LangGo:         "Go",
		LangRust:       "Rust",
		LangZig:        "Zig",
		LangCSharp:     "C#",
		LangJava:       "Java",
		LangKotlin:     "Kotlin",
		LangPHP:        "PHP",
		LangRuby:       "Ruby",
		LangPython:     "Python",
		LangJavaScript: "JavaScript",
		LangTypeScript: "TypeScript",
		LangTSX:        "TSX",
		LangSwift:      "Swift",
		LangBash:       "Bash",
		LangC:          "C",
		LangCPP:        "C++",
		LangJSON:       "JSON",
		LangYAML:       "YAML",
		LangTOML:       "TOML",
	}

	h := NewHighlighter(HighlighterConfig{
		CacheSize:   1,
		Workers:     1,
		DefaultMode: HighlightContextSynthetic,
	})

	out := make([]string, 0, len(h.langs))
	for id, language := range h.langs {
		if language == nil {
			continue
		}
		name, ok := displayNames[id]
		if !ok {
			t.Fatalf("missing README display name for %q", id)
		}
		out = append(out, name)
	}
	return out
}
