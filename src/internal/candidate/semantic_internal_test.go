package candidate

import "testing"

func TestComputeSemanticScoreZigTypeConst(t *testing.T) {
	got := computeSemanticScore("const Mode = enum { fast, slow };")
	if got != semanticTypeDeclScore {
		t.Fatalf("computeSemanticScore(type const) = %d, want %d", got, semanticTypeDeclScore)
	}
}

func TestComputeSemanticScoreZigTest(t *testing.T) {
	got := computeSemanticScore(`test "parses config" {}`)
	if got != semanticFunctionScore {
		t.Fatalf("computeSemanticScore(test) = %d, want %d", got, semanticFunctionScore)
	}
}
