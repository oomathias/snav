package main

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestFilterCandidatesPrefersMatchingCase(t *testing.T) {
	candidates := []Candidate{
		{ID: 1, File: "a.go", Text: "func myFunc() {}", Key: "myFunc"},
		{ID: 2, File: "b.go", Text: "func MyFunc() {}", Key: "MyFunc"},
	}

	res := filterCandidates(candidates, "MyF")
	if len(res) < 2 {
		t.Fatalf("expected at least 2 matches, got %d", len(res))
	}
	if got := candidates[int(res[0].Index)].Key; got != "MyFunc" {
		t.Fatalf("expected MyFunc first for mixed-case query, got %s", got)
	}

	res = filterCandidates(candidates, "myf")
	if len(res) < 2 {
		t.Fatalf("expected at least 2 matches, got %d", len(res))
	}
	if got := candidates[int(res[0].Index)].Key; got != "myFunc" {
		t.Fatalf("expected myFunc first for lowercase query, got %s", got)
	}
}

func TestDefaultRGPatternNamespaceAndClasses(t *testing.T) {
	re := regexp.MustCompile(defaultRGPattern)

	matchCases := []string{
		"namespace Symfind.Core;",
		"inline namespace v1 {",
		"public class SearchIndex : Base {",
		"export default class QueryEngine {",
	}

	for _, tc := range matchCases {
		if !re.MatchString(tc) {
			t.Fatalf("pattern should match %q", tc)
		}
	}

	nonMatchCases := []string{
		"using namespace std;",
		"return className;",
	}

	for _, tc := range nonMatchCases {
		if re.MatchString(tc) {
			t.Fatalf("pattern should not match %q", tc)
		}
	}
}

func TestExtractKeyNamespaceAndClasses(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "dot namespace", text: "namespace Symfind.Core;", want: "Symfind.Core"},
		{name: "cpp namespace", text: "inline namespace symfind::core {", want: "symfind::core"},
		{name: "csharp class", text: "public class SearchIndex : Base {", want: "SearchIndex"},
		{name: "default export class", text: "export default class QueryEngine {", want: "QueryEngine"},
		{name: "final class", text: "final class Tokenizer extends Base {}", want: "Tokenizer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractKey(tt.text, "src/sample.txt")
			if got != tt.want {
				t.Fatalf("ExtractKey(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

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

func TestFilterCandidatesSubsetMatchesFull(t *testing.T) {
	candidates := makeBenchmarkCandidates(8_000)

	baseRaw := trimRunes("hand")
	baseLower := lowerRunes(baseRaw)
	base := filterCandidatesWithQueryRunes(candidates, baseRaw, baseLower)

	nextRaw := trimRunes("handler")
	nextLower := lowerRunes(nextRaw)
	full := filterCandidatesWithQueryRunes(candidates, nextRaw, nextLower)
	subset := filterCandidatesSubsetWithQueryRunes(candidates, base, nextRaw, nextLower)

	if !reflect.DeepEqual(subset, full) {
		t.Fatalf("subset filtering differs from full filtering: subset=%d full=%d", len(subset), len(full))
	}
}

func TestFilterCandidatesParallelMatchesSerial(t *testing.T) {
	candidates := makeBenchmarkCandidates(12_000)
	qRaw := trimRunes("symbol")
	qLower := lowerRunes(qRaw)

	oldThreshold := filterParallelThreshold
	oldChunk := filterMinChunkSize
	defer func() {
		filterParallelThreshold = oldThreshold
		filterMinChunkSize = oldChunk
	}()

	filterParallelThreshold = 1 << 30
	serial := filterCandidatesWithQueryRunes(candidates, qRaw, qLower)

	filterParallelThreshold = 1
	filterMinChunkSize = 1
	parallel := filterCandidatesWithQueryRunes(candidates, qRaw, qLower)

	if !reflect.DeepEqual(parallel, serial) {
		t.Fatalf("parallel filtering differs from serial filtering")
	}
}

func TestFilterCandidatesRangeAndMergeMatchesFull(t *testing.T) {
	candidates := makeBenchmarkCandidates(10_000)
	qRaw := trimRunes("handler")
	qLower := lowerRunes(qRaw)

	split := 6_500
	old := filterCandidatesRangeWithQueryRunes(candidates, 0, split, qRaw, qLower)
	added := filterCandidatesRangeWithQueryRunes(candidates, split, len(candidates), qRaw, qLower)
	merged := mergeFilteredCandidates(candidates, old, added)
	full := filterCandidatesWithQueryRunes(candidates, qRaw, qLower)

	if !reflect.DeepEqual(merged, full) {
		t.Fatalf("range+merge filtering differs from full filtering: merged=%d full=%d", len(merged), len(full))
	}
}
