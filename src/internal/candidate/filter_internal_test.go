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
