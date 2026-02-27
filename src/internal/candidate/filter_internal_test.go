package candidate

import (
	"reflect"
	"testing"
)

func TestFilterCandidatesPrefersMatchingCase(t *testing.T) {
	candidates := []Candidate{
		{ID: 1, File: "a.go", Text: "func myFunc() {}", Key: "myFunc"},
		{ID: 2, File: "b.go", Text: "func MyFunc() {}", Key: "MyFunc"},
	}

	res := FilterCandidates(candidates, "MyF")
	if len(res) < 2 {
		t.Fatalf("expected at least 2 matches, got %d", len(res))
	}
	if got := candidates[int(res[0].Index)].Key; got != "MyFunc" {
		t.Fatalf("expected MyFunc first for mixed-case query, got %s", got)
	}

	res = FilterCandidates(candidates, "myf")
	if len(res) < 2 {
		t.Fatalf("expected at least 2 matches, got %d", len(res))
	}
	if got := candidates[int(res[0].Index)].Key; got != "myFunc" {
		t.Fatalf("expected myFunc first for lowercase query, got %s", got)
	}
}

func TestFilterCandidatesPrefersTypeDeclarationOverLocalVariable(t *testing.T) {
	candidates := []Candidate{
		{ID: 1, File: "cat.ts", Text: "let cat = new Cat()", Key: "cat"},
		{ID: 2, File: "cat.ts", Text: "class Cat {}", Key: "Cat"},
	}

	res := FilterCandidates(candidates, "cat")
	if len(res) < 2 {
		t.Fatalf("expected at least 2 matches, got %d", len(res))
	}
	if got := candidates[int(res[0].Index)].Text; got != "class Cat {}" {
		t.Fatalf("expected type declaration first, got %q", got)
	}
}

func TestFilterCandidatesPrefersContiguousTextMatch(t *testing.T) {
	candidates := []Candidate{
		{ID: 1, File: "billing.ts", Text: "type PolarCheckoutLike = {", Key: "PolarCheckoutLike"},
		{ID: 2, File: "framework.ts", Text: "export type ValidationCheck = {", Key: "ValidationCheck"},
		{ID: 3, File: ".mise.toml", Text: "run = \"bun run typecheck\"", Key: "run"},
	}

	res := FilterCandidates(candidates, "typechec")
	if len(res) < 3 {
		t.Fatalf("expected at least 3 matches, got %d", len(res))
	}
	if got := candidates[int(res[0].Index)].Text; got != "run = \"bun run typecheck\"" {
		t.Fatalf("expected contiguous text match first, got %q", got)
	}
}

func TestFilterCandidatesSubsetMatchesFull(t *testing.T) {
	candidates := makeFixtureCandidates(8_000)

	baseRaw := TrimRunes("hand")
	baseLower := LowerRunes(baseRaw)
	base := FilterCandidatesWithQueryRunes(candidates, baseRaw, baseLower)

	nextRaw := TrimRunes("handler")
	nextLower := LowerRunes(nextRaw)
	full := FilterCandidatesWithQueryRunes(candidates, nextRaw, nextLower)
	subset := FilterCandidatesSubsetWithQueryRunes(candidates, base, nextRaw, nextLower)

	if !reflect.DeepEqual(subset, full) {
		t.Fatalf("subset filtering differs from full filtering: subset=%d full=%d", len(subset), len(full))
	}
}

func TestFilterCandidatesParallelMatchesSerial(t *testing.T) {
	candidates := makeFixtureCandidates(12_000)
	qRaw := TrimRunes("symbol")
	qLower := LowerRunes(qRaw)

	oldThreshold := filterParallelThreshold
	oldChunk := filterMinChunkSize
	defer func() {
		filterParallelThreshold = oldThreshold
		filterMinChunkSize = oldChunk
	}()

	filterParallelThreshold = 1 << 30
	serial := FilterCandidatesWithQueryRunes(candidates, qRaw, qLower)

	filterParallelThreshold = 1
	filterMinChunkSize = 1
	parallel := FilterCandidatesWithQueryRunes(candidates, qRaw, qLower)

	if !reflect.DeepEqual(parallel, serial) {
		t.Fatalf("parallel filtering differs from serial filtering")
	}
}

func TestFilterCandidatesRangeAndMergeMatchesFull(t *testing.T) {
	candidates := makeFixtureCandidates(10_000)
	qRaw := TrimRunes("handler")
	qLower := LowerRunes(qRaw)

	split := 6_500
	old := FilterCandidatesRangeWithQueryRunes(candidates, 0, split, qRaw, qLower)
	added := FilterCandidatesRangeWithQueryRunes(candidates, split, len(candidates), qRaw, qLower)
	merged := MergeFilteredCandidates(candidates, old, added)
	full := FilterCandidatesWithQueryRunes(candidates, qRaw, qLower)

	if !reflect.DeepEqual(merged, full) {
		t.Fatalf("range+merge filtering differs from full filtering: merged=%d full=%d", len(merged), len(full))
	}
}

func TestFilterCandidatesRejectsLooseTextOnlyMatches(t *testing.T) {
	candidates := []Candidate{
		{
			ID:   1,
			File: "README.md",
			Key:  "palette",
			Text: "Type: pickForeground(style, baseFG, chroma.KeywordType, chroma.NameClass), PathDir: pickForeground(style, adjustTone(comment, 0))",
		},
		{
			ID:   2,
			File: "theme.go",
			Key:  "TypeDir",
			Text: "func TypeDir() {}",
		},
	}

	res := FilterCandidates(candidates, "typedir")
	if len(res) != 1 {
		t.Fatalf("expected one match after loose-match rejection, got %d", len(res))
	}
	if got := candidates[int(res[0].Index)].Key; got != "TypeDir" {
		t.Fatalf("expected TypeDir to remain, got %s", got)
	}
}

func TestFilterCandidatesMatchesPathAcrossWhitespaceQuery(t *testing.T) {
	candidates := []Candidate{
		{
			ID:   1,
			File: "src/internal/highlighter/projection.go",
			Key:  "projectSpansToDisplay",
			Text: "func projectSpansToDisplay(baseSpans []Span, sourceLine string, displayLine string) ([]Span, bool) {",
		},
	}

	res := FilterCandidates(candidates, "internal projection")
	if len(res) != 1 {
		t.Fatalf("expected whitespace query to match path, got %d", len(res))
	}
}

func TestFilterCandidatesPathOnlyOpensFirstLine(t *testing.T) {
	candidates := []Candidate{
		{
			ID:   1,
			File: "src/internal/highlighter/projection.go",
			Line: 83,
			Col:  9,
			Key:  "palette",
			Text: "Type: pickForeground(style, baseFG, chroma.KeywordType, chroma.NameClass)",
		},
	}

	res := FilterCandidates(candidates, "internal projection")
	if len(res) != 1 {
		t.Fatalf("expected one path-only match, got %d", len(res))
	}
	if res[0].OpenLine != 1 || res[0].OpenCol != 1 {
		t.Fatalf("expected path-only match to open at 1:1, got %d:%d", res[0].OpenLine, res[0].OpenCol)
	}
}

func TestFilterCandidatesFilenameKeyMatchOpensFirstLine(t *testing.T) {
	candidates := []Candidate{
		{
			ID:   1,
			File: "README.md",
			Line: 18,
			Col:  1,
			Key:  "README",
			Text: "Type a query, pick a result, and open the exact `file:line:col`.",
		},
	}

	res := FilterCandidates(candidates, "README")
	if len(res) != 1 {
		t.Fatalf("expected one filename-key match, got %d", len(res))
	}
	if res[0].OpenLine != 1 || res[0].OpenCol != 1 {
		t.Fatalf("expected filename-key match to open at 1:1, got %d:%d", res[0].OpenLine, res[0].OpenCol)
	}
}
