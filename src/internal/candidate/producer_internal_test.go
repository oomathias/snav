package candidate

import (
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
