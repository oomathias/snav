package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"snav/internal/candidate"
	"snav/internal/highlighter"
	"snav/internal/lang"
)

const llvmUIBatchSize = 2048

func defaultLLVMBenchRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".sources", "llvm-project"))
}

func llvmBenchRoot(b *testing.B) string {
	b.Helper()
	root := strings.TrimSpace(os.Getenv("SNAV_BENCH_ROOT"))
	if root == "" {
		root = defaultLLVMBenchRoot()
	}
	if root == "" {
		b.Skip("set SNAV_BENCH_ROOT or clone llvm-project into .sources/llvm-project")
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

func llvmBenchProducerConfig(root string) candidate.ProducerConfig {
	return candidate.ProducerConfig{
		Root:         root,
		Pattern:      candidate.DefaultRGPattern,
		ExcludeTests: true,
	}
}

func withIndexCachePathForBench(b *testing.B, path string) {
	b.Helper()
	old := indexCachePathOverride
	indexCachePathOverride = path
	b.Cleanup(func() {
		indexCachePathOverride = old
	})
}

func loadCandidatesForRoot(b *testing.B, root string) []candidate.Candidate {
	b.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out, done := candidate.StartProducer(ctx, llvmBenchProducerConfig(root))

	candidates := make([]candidate.Candidate, 0, 200_000)
	for batch := range out {
		candidates = append(candidates, batch...)
	}

	if err, ok := <-done; ok && err != nil {
		b.Fatalf("producer failed: %v", err)
	}
	if len(candidates) == 0 {
		b.Fatalf("producer returned zero candidates")
	}

	return candidates
}

func makeCandidateBatches(candidates []candidate.Candidate, batchSize int) [][]candidate.Candidate {
	if batchSize <= 0 {
		batchSize = 1
	}
	batches := make([][]candidate.Candidate, 0, (len(candidates)+batchSize-1)/batchSize)
	for start := 0; start < len(candidates); start += batchSize {
		end := start + batchSize
		if end > len(candidates) {
			end = len(candidates)
		}
		batch := make([]candidate.Candidate, end-start)
		copy(batch, candidates[start:end])
		batches = append(batches, batch)
	}
	return batches
}

func benchmarkLLVMUIDrain(b *testing.B, maxItems int) {
	root := llvmBenchRoot(b)
	candidates := loadCandidatesForRoot(b, root)
	batches := makeCandidateBatches(candidates, llvmUIBatchSize)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		out := make(chan []candidate.Candidate, len(batches))
		for _, batch := range batches {
			out <- batch
		}
		close(out)

		m := model{producerOut: out}
		ticks := 0
		for m.producerOut != nil {
			m.drainProducer(maxItems)
			ticks++
		}
		if len(m.candidates) != len(candidates) {
			b.Fatalf("drained %d candidates, want %d", len(m.candidates), len(candidates))
		}
		b.ReportMetric(float64(ticks), "ticks/op")
	}
}

func benchmarkLLVMStartupLoop(b *testing.B, preview bool, withHighlighter bool) {
	root := llvmBenchRoot(b)
	candidates := loadCandidatesForRoot(b, root)
	batches := makeCandidateBatches(candidates, llvmUIBatchSize)

	var hl *highlighter.Highlighter
	if withHighlighter {
		hl = highlighter.NewHighlighter(highlighter.HighlighterConfig{
			CacheSize:     20000,
			Workers:       1,
			Root:          root,
			DefaultMode:   highlighter.HighlightContextFile,
			ContextRadius: 40,
		})
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		out := make(chan []candidate.Candidate, len(batches))
		for _, batch := range batches {
			out <- batch
		}
		close(out)

		m := model{
			cfg: config{
				Root:          root,
				Preview:       preview,
				VisibleBuffer: 30,
				HighlightMode: highlighter.HighlightContextFile,
				ContextRadius: 40,
			},
			width:          160,
			height:         48,
			producerOut:    out,
			highlighter:    hl,
			previewEnabled: preview,
			fileCache:      make(map[string][]string),
			fileLangCache:  make(map[string]highlighter.LangID),
		}

		ticks := 0
		for m.producerOut != nil {
			m.drainProducer(m.producerDrainLimit())
			m.drainProducerDone()
			m.ensureCursor()
			m.updatePreview()
			m.queueVisibleHighlights()
			ticks++
		}

		if len(m.candidates) != len(candidates) {
			b.Fatalf("startup loop drained %d candidates, want %d", len(m.candidates), len(candidates))
		}
		b.ReportMetric(float64(ticks), "ticks/op")
	}
}

func llvmBenchHotFile(root string) string {
	return filepath.Join(root, "llvm", "include", "llvm", "ADT", "StringRef.h")
}

func loadCandidatesForFile(b *testing.B, root string, file string) []candidate.Candidate {
	b.Helper()

	rel, err := filepath.Rel(root, file)
	if err != nil {
		b.Fatalf("relative path: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	args := []string{
		"--vimgrep",
		"--null",
		"--trim",
		"--color", "never",
		"--no-heading",
		"--smart-case",
		candidate.DefaultRGPattern,
		rel,
	}
	cmd := exec.CommandContext(ctx, "rg", args...)
	cmd.Dir = root

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		b.Fatalf("open rg stdout: %v", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		b.Fatalf("start rg: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 128*1024), 8*1024*1024)

	candidates := make([]candidate.Candidate, 0, 64)
	for scanner.Scan() {
		path, lineNo, colNo, text, ok := parseBenchRGVimgrepLine(scanner.Bytes())
		if !ok {
			continue
		}

		cleanFile := filepath.Clean(path)
		candidates = append(candidates, candidate.Candidate{
			ID:            len(candidates) + 1,
			File:          cleanFile,
			Line:          lineNo,
			Col:           colNo,
			Text:          text,
			Key:           candidate.ExtractKey(text, cleanFile),
			LangID:        lang.Detect(cleanFile),
			SemanticScore: 0,
		})
	}

	if err := scanner.Err(); err != nil {
		b.Fatalf("read rg output: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		b.Fatalf("rg failed: %v (%s)", err, strings.TrimSpace(stderr.String()))
	}
	if len(candidates) == 0 {
		b.Fatalf("single-file scan returned zero candidates for %s", rel)
	}

	return candidates
}

func parseBenchRGVimgrepLine(raw []byte) (file string, lineNo int, colNo int, text string, ok bool) {
	nul := bytes.IndexByte(raw, 0)
	if nul <= 0 || nul >= len(raw)-1 {
		return "", 0, 0, "", false
	}

	file = string(raw[:nul])
	rest := raw[nul+1:]

	parsedLine, rest, ok := parseBenchPositiveIntField(rest)
	if !ok {
		return "", 0, 0, "", false
	}
	parsedCol, rest, ok := parseBenchPositiveIntField(rest)
	if !ok {
		return "", 0, 0, "", false
	}

	lineNo = parsedLine
	colNo = parsedCol
	text = strings.TrimRight(string(rest), "\r")
	return file, lineNo, colNo, text, true
}

func parseBenchPositiveIntField(raw []byte) (int, []byte, bool) {
	sep := bytes.IndexByte(raw, ':')
	if sep <= 0 {
		return 0, nil, false
	}

	value := 0
	for _, b := range raw[:sep] {
		if b < '0' || b > '9' {
			return 0, nil, false
		}
		value = value*10 + int(b-'0')
	}
	if value <= 0 {
		return 0, nil, false
	}

	return value, raw[sep+1:], true
}

func replaceCandidatesForFile(all []candidate.Candidate, file string, replacement []candidate.Candidate) []candidate.Candidate {
	out := make([]candidate.Candidate, 0, len(all)+len(replacement))

	for i := range all {
		if all[i].File == file {
			continue
		}
		cand := all[i]
		cand.ID = len(out) + 1
		out = append(out, cand)
	}

	for i := range replacement {
		cand := replacement[i]
		cand.ID = len(out) + 1
		out = append(out, cand)
	}

	return out
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

func BenchmarkLLVMIndexCacheSave(b *testing.B) {
	root := llvmBenchRoot(b)
	cfg := llvmBenchProducerConfig(root)
	candidates := loadCandidatesForRoot(b, root)
	cachePath := filepath.Join(b.TempDir(), "last_index.gob")
	withIndexCachePathForBench(b, cachePath)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := SaveIndexCache(cfg, candidates); err != nil {
			b.Fatalf("SaveIndexCache failed: %v", err)
		}
	}

	b.ReportMetric(float64(len(candidates)), "cands/op")
}

func BenchmarkLLVMIndexCacheLoad(b *testing.B) {
	root := llvmBenchRoot(b)
	cfg := llvmBenchProducerConfig(root)
	candidates := loadCandidatesForRoot(b, root)
	cachePath := filepath.Join(b.TempDir(), "last_index.gob")
	withIndexCachePathForBench(b, cachePath)

	if err := SaveIndexCache(cfg, candidates); err != nil {
		b.Fatalf("SaveIndexCache setup failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		got, ok, err := LoadIndexCache(cfg)
		if err != nil {
			b.Fatalf("LoadIndexCache failed: %v", err)
		}
		if !ok {
			b.Fatalf("LoadIndexCache returned cache miss")
		}
		if len(got) != len(candidates) {
			b.Fatalf("LoadIndexCache returned %d candidates, want %d", len(got), len(candidates))
		}
	}

	b.ReportMetric(float64(len(candidates)), "cands/op")
}

func BenchmarkLLVMWarmStartFirstQuery(b *testing.B) {
	root := llvmBenchRoot(b)
	cfg := llvmBenchProducerConfig(root)
	candidates := loadCandidatesForRoot(b, root)
	cachePath := filepath.Join(b.TempDir(), "last_index.gob")
	withIndexCachePathForBench(b, cachePath)
	query := "LLVMContext"

	if err := SaveIndexCache(cfg, candidates); err != nil {
		b.Fatalf("SaveIndexCache setup failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		got, ok, err := LoadIndexCache(cfg)
		if err != nil {
			b.Fatalf("LoadIndexCache failed: %v", err)
		}
		if !ok {
			b.Fatalf("LoadIndexCache returned cache miss")
		}

		matches := candidate.FilterCandidates(got, query)
		if len(matches) == 0 {
			b.Fatalf("query %q returned zero matches", query)
		}
	}

	b.ReportMetric(float64(len(candidates)), "cands/op")
}

func BenchmarkLLVMIncrementalReindexOneFile(b *testing.B) {
	root := llvmBenchRoot(b)
	hotFile := llvmBenchHotFile(root)

	if _, err := os.Stat(hotFile); err != nil {
		b.Fatalf("hot file missing: %v", err)
	}

	warm := loadCandidatesForFile(b, root, hotFile)
	b.ReportMetric(float64(len(warm)), "cands/file")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		candidates := loadCandidatesForFile(b, root, hotFile)
		if len(candidates) == 0 {
			b.Fatalf("incremental reindex returned zero candidates")
		}
	}

	b.ReportMetric(1, fmt.Sprintf("file:%s", filepath.Base(hotFile)))
}

func BenchmarkLLVMEditOneFileRebuildVisibleQuery(b *testing.B) {
	root := llvmBenchRoot(b)
	hotFile := llvmBenchHotFile(root)
	relHotFile, err := filepath.Rel(root, hotFile)
	if err != nil {
		b.Fatalf("relative hot file path: %v", err)
	}

	candidates := loadCandidatesForRoot(b, root)
	query := "StringRef"
	warmMatches := candidate.FilterCandidates(candidates, query)
	if len(warmMatches) == 0 {
		b.Fatalf("warm query %q returned zero matches", query)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		replacement := loadCandidatesForFile(b, root, hotFile)
		updated := replaceCandidatesForFile(candidates, relHotFile, replacement)
		matches := candidate.FilterCandidates(updated, query)
		if len(matches) == 0 {
			b.Fatalf("query %q returned zero matches after file rebuild", query)
		}
	}

	b.ReportMetric(float64(len(candidates)), "cands/op")
	b.ReportMetric(float64(len(warmMatches)), "matches/op")
}

func BenchmarkLLVMUIDrain4000(b *testing.B) {
	benchmarkLLVMUIDrain(b, 4000)
}

func BenchmarkLLVMUIDrain65536(b *testing.B) {
	benchmarkLLVMUIDrain(b, 65536)
}

func BenchmarkLLVMUIDrainUnlimited(b *testing.B) {
	benchmarkLLVMUIDrain(b, 0)
}

func BenchmarkLLVMUIDrainStartupPolicy(b *testing.B) {
	benchmarkLLVMUIDrain(b, producerDrainItemsStartup)
}

func BenchmarkLLVMStartupLoopNoPreview(b *testing.B) {
	benchmarkLLVMStartupLoop(b, false, false)
}

func BenchmarkLLVMStartupLoopDefaultUI(b *testing.B) {
	benchmarkLLVMStartupLoop(b, true, true)
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
		matches := candidate.FilterCandidates(candidates, queries[i%len(queries)])
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
		rawQueries[i] = candidate.TrimRunes(queries[i])
		lowerQueries[i] = candidate.LowerRunes(rawQueries[i])
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		matches := candidate.FilterCandidatesWithQueryRunes(candidates, rawQueries[0], lowerQueries[0])
		for j := 1; j < len(rawQueries); j++ {
			matches = candidate.FilterCandidatesSubsetWithQueryRunes(candidates, matches, rawQueries[j], lowerQueries[j])
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
		rawQueries[i] = candidate.TrimRunes(queries[i])
		lowerQueries[i] = candidate.LowerRunes(rawQueries[i])
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var matches []candidate.FilteredCandidate
		for j := range rawQueries {
			matches = candidate.FilterCandidatesWithQueryRunes(candidates, rawQueries[j], lowerQueries[j])
		}
		if len(matches) == 0 {
			b.Fatalf("typeahead full sequence returned zero matches")
		}
	}
}

func BenchmarkLLVMFilterStreamingAppend(b *testing.B) {
	root := llvmBenchRoot(b)
	candidates := loadCandidatesForRoot(b, root)
	qRaw := candidate.TrimRunes("llvmcontext")
	qLower := candidate.LowerRunes(qRaw)
	const batchSize = 4_000

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var matches []candidate.FilteredCandidate
		for start := 0; start < len(candidates); start += batchSize {
			end := min(len(candidates), start+batchSize)
			added := candidate.FilterCandidatesRangeWithQueryRunes(candidates, start, end, qRaw, qLower)
			matches = candidate.MergeFilteredCandidates(candidates, matches, added)
		}
		if len(matches) == 0 {
			b.Fatalf("streaming append sequence returned zero matches")
		}
	}
}

func BenchmarkLLVMFilterStreamingFullRescan(b *testing.B) {
	root := llvmBenchRoot(b)
	candidates := loadCandidatesForRoot(b, root)
	qRaw := candidate.TrimRunes("llvmcontext")
	qLower := candidate.LowerRunes(qRaw)
	const batchSize = 4_000

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var matches []candidate.FilteredCandidate
		for end := batchSize; end < len(candidates); end += batchSize {
			matches = candidate.FilterCandidatesWithQueryRunes(candidates[:end], qRaw, qLower)
		}
		if len(matches) == 0 {
			b.Fatalf("streaming full-rescan sequence returned zero matches")
		}
	}
}
