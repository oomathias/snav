package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

func llvmBenchRoot(b *testing.B) string {
	b.Helper()
	root := strings.TrimSpace(os.Getenv("SNAV_BENCH_ROOT"))
	if root == "" {
		b.Skip("set SNAV_BENCH_ROOT to run LLVM benchmarks")
	}
	info, err := os.Stat(root)
	if err != nil {
		b.Skipf("cannot access SNAV_BENCH_ROOT %q: %v", root, err)
	}
	if !info.IsDir() {
		b.Skipf("SNAV_BENCH_ROOT %q is not a directory", root)
	}
	return root
}

func loadCandidatesForRoot(b *testing.B, root string) []Candidate {
	b.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out, done := StartProducer(ctx, ProducerConfig{
		Root:         root,
		Pattern:      defaultRGPattern,
		ExcludeTests: true,
	})

	candidates := make([]Candidate, 0, 200_000)
	for cand := range out {
		candidates = append(candidates, cand)
	}

	if err, ok := <-done; ok && err != nil {
		b.Fatalf("producer failed: %v", err)
	}
	if len(candidates) == 0 {
		b.Fatalf("producer returned zero candidates")
	}

	return candidates
}

func BenchmarkLLVMProducerScan(b *testing.B) {
	root := llvmBenchRoot(b)
	b.ReportAllocs()

	total := 0
	for i := 0; i < b.N; i++ {
		candidates := loadCandidatesForRoot(b, root)
		total += len(candidates)
	}

	b.ReportMetric(float64(total)/float64(b.N), "cands/op")
}

func BenchmarkLLVMFilterQueries(b *testing.B) {
	root := llvmBenchRoot(b)
	candidates := loadCandidatesForRoot(b, root)
	queries := []string{
		"LLVMContext",
		"DenseMap",
		"PassManager",
		"APInt",
		"SmallVector",
		"StringRef",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		matches := filterCandidates(candidates, queries[i%len(queries)])
		if len(matches) == 0 {
			b.Fatalf("query %q returned zero matches", queries[i%len(queries)])
		}
	}
}

func BenchmarkLLVMFilterTypeahead(b *testing.B) {
	root := llvmBenchRoot(b)
	candidates := loadCandidatesForRoot(b, root)
	queries := []string{
		"l",
		"ll",
		"llvm",
		"llvmc",
		"llvmco",
		"llvmcon",
		"llvmcont",
		"llvmconte",
		"llvmcontex",
		"llvmcontext",
	}

	rawQueries := make([][]rune, len(queries))
	lowerQueries := make([][]rune, len(queries))
	for i := range queries {
		rawQueries[i] = trimRunes(queries[i])
		lowerQueries[i] = lowerRunes(rawQueries[i])
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		matches := filterCandidatesWithQueryRunes(candidates, rawQueries[0], lowerQueries[0])
		for j := 1; j < len(rawQueries); j++ {
			matches = filterCandidatesSubsetWithQueryRunes(candidates, matches, rawQueries[j], lowerQueries[j])
		}
		if len(matches) == 0 {
			b.Fatalf("typeahead sequence returned zero matches")
		}
	}
}

func BenchmarkLLVMFilterTypeaheadFull(b *testing.B) {
	root := llvmBenchRoot(b)
	candidates := loadCandidatesForRoot(b, root)
	queries := []string{
		"l",
		"ll",
		"llvm",
		"llvmc",
		"llvmco",
		"llvmcon",
		"llvmcont",
		"llvmconte",
		"llvmcontex",
		"llvmcontext",
	}

	rawQueries := make([][]rune, len(queries))
	lowerQueries := make([][]rune, len(queries))
	for i := range queries {
		rawQueries[i] = trimRunes(queries[i])
		lowerQueries[i] = lowerRunes(rawQueries[i])
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var matches []filteredCandidate
		for j := range rawQueries {
			matches = filterCandidatesWithQueryRunes(candidates, rawQueries[j], lowerQueries[j])
		}
		if len(matches) == 0 {
			b.Fatalf("typeahead full sequence returned zero matches")
		}
	}
}

func BenchmarkLLVMFilterStreamingAppend(b *testing.B) {
	root := llvmBenchRoot(b)
	candidates := loadCandidatesForRoot(b, root)
	qRaw := trimRunes("llvmcontext")
	qLower := lowerRunes(qRaw)
	const batchSize = 4_000

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var matches []filteredCandidate
		for start := 0; start < len(candidates); start += batchSize {
			end := min(len(candidates), start+batchSize)
			added := filterCandidatesRangeWithQueryRunes(candidates, start, end, qRaw, qLower)
			matches = mergeFilteredCandidates(candidates, matches, added)
		}
		if len(matches) == 0 {
			b.Fatalf("streaming append sequence returned zero matches")
		}
	}
}

func BenchmarkLLVMFilterStreamingFullRescan(b *testing.B) {
	root := llvmBenchRoot(b)
	candidates := loadCandidatesForRoot(b, root)
	qRaw := trimRunes("llvmcontext")
	qLower := lowerRunes(qRaw)
	const batchSize = 4_000

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var matches []filteredCandidate
		for end := batchSize; end < len(candidates); end += batchSize {
			matches = filterCandidatesWithQueryRunes(candidates[:end], qRaw, qLower)
		}
		if len(matches) == 0 {
			b.Fatalf("streaming full-rescan sequence returned zero matches")
		}
	}
}
