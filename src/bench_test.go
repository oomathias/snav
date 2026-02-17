package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func BenchmarkFilterCandidates50k(b *testing.B) {
	b.ReportAllocs()
	candidates := makeBenchmarkCandidates(50_000)
	queries := []string{"parse", "handler", "json", "snav"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = filterCandidates(candidates, queries[i%len(queries)])
	}
}

func BenchmarkHighlightSyntheticLine(b *testing.B) {
	b.ReportAllocs()
	h := NewHighlighter(HighlighterConfig{
		CacheSize:   256,
		Workers:     1,
		DefaultMode: HighlightContextSynthetic,
	})
	parser := sitter.NewParser()
	req := HighlightRequest{
		Lang: LangTypeScript,
		Text: `const value = user.profile?.displayName ?? "unknown"`,
		Mode: HighlightContextSynthetic,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.highlightWithParser(parser, req)
	}
}

func BenchmarkHighlightFileContextLine(b *testing.B) {
	b.ReportAllocs()
	dir := b.TempDir()
	source, targetLine, targetText := makeBenchmarkGoSource(360)
	path := filepath.Join(dir, "sample.go")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		b.Fatalf("write sample file: %v", err)
	}

	h := NewHighlighter(HighlighterConfig{
		CacheSize:     256,
		Workers:       1,
		Root:          dir,
		DefaultMode:   HighlightContextFile,
		ContextRadius: 40,
	})
	parser := sitter.NewParser()
	req := HighlightRequest{
		Lang: LangGo,
		Text: targetText,
		File: path,
		Line: targetLine,
		Mode: HighlightContextFile,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.highlightWithParser(parser, req)
	}
}

func makeBenchmarkCandidates(n int) []Candidate {
	out := make([]Candidate, n)
	for i := 0; i < n; i++ {
		lang := LangGo
		file := fmt.Sprintf("pkg/mod%d/file%d.go", i%100, i%37)
		text := fmt.Sprintf("func Symbol%dHandler(input%d int) int { return input%d + %d }", i, i, i, i%11)

		switch i % 4 {
		case 1:
			lang = LangTypeScript
			file = fmt.Sprintf("src/mod%d/file%d.ts", i%90, i%45)
			text = fmt.Sprintf("export const symbol%dHandler = (input%d: number) => input%d + %d", i, i, i, i%13)
		case 2:
			lang = LangRust
			file = fmt.Sprintf("crates/mod%d/file%d.rs", i%70, i%29)
			text = fmt.Sprintf("pub fn symbol%d_handler(input%d: i64) -> i64 { input%d + %d }", i, i, i, i%17)
		case 3:
			lang = LangPython
			file = fmt.Sprintf("py/mod%d/file%d.py", i%60, i%31)
			text = fmt.Sprintf("def symbol_%d_handler(input_%d): return input_%d + %d", i, i, i, i%19)
		}

		out[i] = Candidate{
			ID:     i + 1,
			File:   file,
			Line:   (i % 400) + 1,
			Col:    1,
			Text:   text,
			Key:    fmt.Sprintf("Symbol%dHandler", i),
			LangID: lang,
		}
	}
	return out
}

func makeBenchmarkGoSource(bodyLines int) (string, int, string) {
	var sb strings.Builder
	sb.WriteString("package bench\n\n")
	sb.WriteString("func run(items []int) int {\n")

	targetLine := 0
	targetText := ""
	for i := 0; i < bodyLines; i++ {
		line := fmt.Sprintf("\tvalue%d := items[%d%%len(items)] + %d", i, i, i)
		if i == bodyLines/2 {
			targetLine = 4 + i
			targetText = line
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	sb.WriteString("\treturn value0\n")
	sb.WriteString("}\n")
	return sb.String(), targetLine, targetText
}
