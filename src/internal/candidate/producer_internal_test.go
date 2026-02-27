package candidate

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseRGVimgrepLineWithColonsInPathAndText(t *testing.T) {
	line := []byte("pkg:2024:10:module/file.go\x0041:9:12:34 payload")

	file, lineNo, colNo, text, ok := parseRGVimgrepLine(line)
	if !ok {
		t.Fatalf("parseRGVimgrepLine should parse match line")
	}
	if file != "pkg:2024:10:module/file.go" {
		t.Fatalf("file = %q", file)
	}
	if lineNo != 41 || colNo != 9 {
		t.Fatalf("line:col = %d:%d, want 41:9", lineNo, colNo)
	}
	if text != "12:34 payload" {
		t.Fatalf("text = %q", text)
	}
}

func TestParseRGVimgrepLineRejectsMalformedInput(t *testing.T) {
	tests := [][]byte{
		[]byte(""),
		[]byte("path:41:9:text"),
		[]byte("path\x00x:9:text"),
		[]byte("path\x0041:y:text"),
		[]byte("path\x0041:9"),
	}

	for _, tc := range tests {
		_, _, _, _, ok := parseRGVimgrepLine(tc)
		if ok {
			t.Fatalf("parseRGVimgrepLine should reject %q", tc)
		}
	}
}

func TestTestExcludeGlobsAreSpecific(t *testing.T) {
	for _, glob := range testExcludeGlobs {
		if strings.Contains(glob, "*test*") || strings.Contains(glob, "*spec*") {
			t.Fatalf("glob %q is too broad and can hide non-test files", glob)
		}
	}
}

func TestRGArgs(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg := ProducerConfig{
			Pattern: "",
		}
		got := rgArgs(cfg, "todo")
		want := []string{
			"--vimgrep",
			"--null",
			"--trim",
			"--color", "never",
			"--no-heading",
			"--smart-case",
			"todo", ".",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("rgArgs = %#v, want %#v", got, want)
		}
	})

	t.Run("flags", func(t *testing.T) {
		cfg := ProducerConfig{
			NoIgnore:     true,
			Excludes:     []string{"a/**", "b/**"},
			ExcludeTests: true,
			Pattern:      "func",
		}
		got := rgArgs(cfg, "func")
		want := []string{
			"--vimgrep",
			"--null",
			"--trim",
			"--color", "never",
			"--no-heading",
			"--smart-case",
			"--no-ignore",
			"--glob", "!a/**",
			"--glob", "!b/**",
			"--glob", "!test/**",
			"--glob", "!tests/**",
			"--glob", "!__tests__/**",
			"--glob", "!spec/**",
			"--glob", "!specs/**",
			"--glob", "!**/test/**",
			"--glob", "!**/tests/**",
			"--glob", "!**/__tests__/**",
			"--glob", "!**/spec/**",
			"--glob", "!**/specs/**",
			"--glob", "!*_test.*",
			"--glob", "!*_spec.*",
			"--glob", "!*.test.*",
			"--glob", "!*.spec.*",
			"--glob", "!test_*.py",
			"--glob", "!**/*_test.*",
			"--glob", "!**/*_spec.*",
			"--glob", "!**/*.test.*",
			"--glob", "!**/*.spec.*",
			"--glob", "!**/test_*.py",
			"func",
			".",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("rgArgs = %#v, want %#v", got, want)
		}
	})
}
