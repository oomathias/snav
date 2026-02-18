package main_test

import (
	"reflect"
	"testing"

	"snav/internal/candidate"
)

func TestCandidateFilterCandidatesEmptyQueryReturnsAll(t *testing.T) {
	candidates := []candidate.Candidate{
		{ID: 1, File: "a.go", Key: "Alpha", Text: "func Alpha() {}"},
		{ID: 2, File: "b.go", Key: "Beta", Text: "func Beta() {}"},
	}

	got := candidate.FilterCandidates(candidates, "")
	if len(got) != len(candidates) {
		t.Fatalf("expected %d candidates for empty query, got %d", len(candidates), len(got))
	}
}

func TestCandidateRangeMergeMatchesFull(t *testing.T) {
	candidates := []candidate.Candidate{
		{ID: 1, File: "a.go", Key: "AlphaHandler", Text: "func AlphaHandler() {}"},
		{ID: 2, File: "b.go", Key: "BetaHandler", Text: "func BetaHandler() {}"},
		{ID: 3, File: "c.go", Key: "Gamma", Text: "func Gamma() {}"},
		{ID: 4, File: "d.go", Key: "DeltaHandler", Text: "func DeltaHandler() {}"},
	}

	qRaw := candidate.TrimRunes("handler")
	qLower := candidate.LowerRunes(qRaw)

	left := candidate.FilterCandidatesRangeWithQueryRunes(candidates, 0, 2, qRaw, qLower)
	right := candidate.FilterCandidatesRangeWithQueryRunes(candidates, 2, len(candidates), qRaw, qLower)
	merged := candidate.MergeFilteredCandidates(candidates, left, right)
	full := candidate.FilterCandidatesWithQueryRunes(candidates, qRaw, qLower)

	if !reflect.DeepEqual(merged, full) {
		t.Fatalf("merged results do not match full results")
	}
}
