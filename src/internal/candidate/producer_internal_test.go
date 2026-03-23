package candidate

import (
	"context"
	"os"
	"path/filepath"
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
	t.Run("default pattern adds declaration globs", func(t *testing.T) {
		cfg := ProducerConfig{}
		got := rgArgs(cfg, DefaultRGPattern)

		want := []string{
			"--vimgrep",
			"--null",
			"--trim",
			"--color", "never",
			"--no-heading",
			"--smart-case",
		}
		for _, glob := range declarationIncludeGlobs {
			want = append(want, "--glob", glob)
		}
		want = append(want, DefaultRGPattern)

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("rgArgs = %#v, want %#v", got, want)
		}
	})

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
			"todo",
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
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("rgArgs = %#v, want %#v", got, want)
		}
	})
}

func TestRGConfigArgs(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg := ProducerConfig{}
		got := rgConfigArgs(cfg)
		want := []string{
			"--vimgrep",
			"--null",
			"--trim",
			"--color", "never",
			"--no-heading",
			"--smart-case",
			"--glob", "*.json",
			"--glob", "*.jsonc",
			"--glob", "*.json5",
			"--glob", "*.yaml",
			"--glob", "*.yml",
			"--glob", "*.toml",
			"--glob", "*.ini",
			"--glob", ".env",
			"--glob", ".env.*",
			"--glob", ".envrc",
			"--glob", "*.properties",
			"--glob", "*.conf",
			"--glob", "*.cfg",
			"--glob", "*.cnf",
			"--glob", "*.tf",
			"--glob", "*.hcl",
			"--glob", "*.tfvars",
			"--glob", "*.xml",
			"--glob", "*.plist",
			"--glob", "*.csproj",
			"--glob", "*.props",
			"--glob", "*.targets",
			"--glob", "*.config",
			DefaultRGConfigPattern,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("rgConfigArgs = %#v, want %#v", got, want)
		}
	})

	t.Run("flags", func(t *testing.T) {
		cfg := ProducerConfig{
			NoIgnore:     true,
			Excludes:     []string{"a/**", "b/**"},
			ExcludeTests: true,
		}
		got := rgConfigArgs(cfg)
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
			"--glob", "*.json",
			"--glob", "*.jsonc",
			"--glob", "*.json5",
			"--glob", "*.yaml",
			"--glob", "*.yml",
			"--glob", "*.toml",
			"--glob", "*.ini",
			"--glob", ".env",
			"--glob", ".env.*",
			"--glob", ".envrc",
			"--glob", "*.properties",
			"--glob", "*.conf",
			"--glob", "*.cfg",
			"--glob", "*.cnf",
			"--glob", "*.tf",
			"--glob", "*.hcl",
			"--glob", "*.tfvars",
			"--glob", "*.xml",
			"--glob", "*.plist",
			"--glob", "*.csproj",
			"--glob", "*.props",
			"--glob", "*.targets",
			"--glob", "*.config",
			DefaultRGConfigPattern,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("rgConfigArgs = %#v, want %#v", got, want)
		}
	})
}

func TestStartProducerDefaultPatternFindsSwiftDeclarations(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "App", "Core", "Service.swift")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	src := `import Foundation

final class ServiceManager {
  func install(configPath: String) throws {}
}
`
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	out, done := StartProducer(context.Background(), ProducerConfig{Root: root})

	var got []Candidate
	for batch := range out {
		got = append(got, batch...)
	}

	if err := <-done; err != nil {
		t.Fatalf("StartProducer error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].File != filepath.ToSlash(filepath.Join("App", "Core", "Service.swift")) {
		t.Fatalf("got[0].File = %q", got[0].File)
	}
	if got[0].Key != "ServiceManager" {
		t.Fatalf("got[0].Key = %q, want %q", got[0].Key, "ServiceManager")
	}
	if got[1].Key != "install" {
		t.Fatalf("got[1].Key = %q, want %q", got[1].Key, "install")
	}
}

func TestIsNoFilesSearchedMessage(t *testing.T) {
	msg := "rg: No files were searched, which means ripgrep probably applied a filter you didn't expect.\nRunning with --debug will show why files are being skipped."
	if !isNoFilesSearchedMessage(msg) {
		t.Fatalf("expected no-files-searched message to be detected")
	}
	if isNoFilesSearchedMessage("rg: regex parse error:\n    (\n    ^\nerror: unclosed group") {
		t.Fatalf("unexpected match for unrelated rg error")
	}
}

func TestShouldIncludeConfigPass(t *testing.T) {
	if !shouldIncludeConfigPass(DefaultRGPattern) {
		t.Fatalf("default pattern should include config pass")
	}
	if shouldIncludeConfigPass("^foo$") {
		t.Fatalf("custom pattern should not include config pass")
	}
}
