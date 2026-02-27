package candidate

import (
	"runtime"
	"sort"
	"strings"
	"sync"
)

func FilterCandidates(candidates []Candidate, query string) []FilteredCandidate {
	q := TrimRunes(query)
	return FilterCandidatesWithQueryRunes(candidates, q, LowerRunes(q))
}

func FilterCandidatesWithRunes(candidates []Candidate, q []rune) []FilteredCandidate {
	return FilterCandidatesWithQueryRunes(candidates, nil, q)
}

func FilterCandidatesSubsetWithQueryRunes(candidates []Candidate, subset []FilteredCandidate, qRaw []rune, qLower []rune) []FilteredCandidate {
	return filterCandidatesCore(candidates, subset, qRaw, qLower)
}

func FilterCandidatesRangeWithQueryRunes(candidates []Candidate, start int, end int, qRaw []rune, qLower []rune) []FilteredCandidate {
	if start < 0 {
		start = 0
	}
	if end > len(candidates) {
		end = len(candidates)
	}
	if start >= end {
		return nil
	}

	caseSensitive := len(qRaw) == len(qLower)
	n := end - start
	workers := filterWorkerCount(n)
	var out []FilteredCandidate
	if workers <= 1 {
		out = make([]FilteredCandidate, 0, max(1, n/4))
		out = appendScoredRange(out, candidates, nil, start, end, qRaw, qLower, caseSensitive)
	} else {
		out = filterCandidatesParallelChunks(workers, n, func(chunkStart int, chunkEnd int) []FilteredCandidate {
			local := make([]FilteredCandidate, 0, max(1, (chunkEnd-chunkStart)/4))
			return appendScoredRange(local, candidates, nil, start+chunkStart, start+chunkEnd, qRaw, qLower, caseSensitive)
		})
	}

	sortFilteredCandidates(candidates, out)
	return out
}

func FilterCandidatesWithQueryRunes(candidates []Candidate, qRaw []rune, qLower []rune) []FilteredCandidate {
	return filterCandidatesCore(candidates, nil, qRaw, qLower)
}

func filterCandidatesCore(candidates []Candidate, subset []FilteredCandidate, qRaw []rune, qLower []rune) []FilteredCandidate {
	if len(qLower) == 0 {
		out := make([]FilteredCandidate, len(candidates))
		for i := range candidates {
			out[i] = FilteredCandidate{Index: int32(i)}
		}
		return out
	}
	if subset != nil && len(subset) == 0 {
		return nil
	}

	caseSensitive := len(qRaw) == len(qLower)
	rangeLen := len(candidates)
	serialCapacity := len(candidates) / 4
	parallelDivisor := 4
	if subset != nil {
		rangeLen = len(subset)
		serialCapacity = len(subset) / 2
		parallelDivisor = 2
	}

	workers := filterWorkerCount(rangeLen)
	var out []FilteredCandidate
	if workers <= 1 {
		out = make([]FilteredCandidate, 0, serialCapacity)
		out = appendScoredRange(out, candidates, subset, 0, rangeLen, qRaw, qLower, caseSensitive)
	} else {
		out = filterCandidatesParallelChunks(workers, rangeLen, func(start int, end int) []FilteredCandidate {
			local := make([]FilteredCandidate, 0, max(1, (end-start)/parallelDivisor))
			return appendScoredRange(local, candidates, subset, start, end, qRaw, qLower, caseSensitive)
		})
	}

	sortFilteredCandidates(candidates, out)
	return out
}

func filterWorkerCount(n int) int {
	if n < filterParallelThreshold {
		return 1
	}

	workers := runtime.GOMAXPROCS(0)
	if workers < 2 {
		return 1
	}

	maxUseful := n / filterMinChunkSize
	if maxUseful < 2 {
		return 1
	}
	if workers > maxUseful {
		workers = maxUseful
	}

	return workers
}

func appendScoredRange(out []FilteredCandidate, candidates []Candidate, subset []FilteredCandidate, start int, end int, qRaw []rune, qLower []rune, caseSensitive bool) []FilteredCandidate {
	if subset == nil {
		for i := start; i < end; i++ {
			item, ok := scoreCandidate(&candidates[i], int32(i), qRaw, qLower, caseSensitive)
			if !ok {
				continue
			}
			out = append(out, item)
		}
		return out
	}

	for i := start; i < end; i++ {
		idx := int(subset[i].Index)
		if idx < 0 || idx >= len(candidates) {
			continue
		}

		item, ok := scoreCandidate(&candidates[idx], subset[i].Index, qRaw, qLower, caseSensitive)
		if !ok {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterCandidatesParallelChunks(workers int, n int, filterChunk func(start int, end int) []FilteredCandidate) []FilteredCandidate {
	parts := make([][]FilteredCandidate, workers)
	var wg sync.WaitGroup

	for worker := 0; worker < workers; worker++ {
		start := worker * n / workers
		end := (worker + 1) * n / workers
		wg.Add(1)
		go func(slot int, start int, end int) {
			defer wg.Done()
			parts[slot] = filterChunk(start, end)
		}(worker, start, end)
	}

	wg.Wait()
	return flattenFilteredParts(parts)
}

func flattenFilteredParts(parts [][]FilteredCandidate) []FilteredCandidate {
	total := 0
	for _, part := range parts {
		total += len(part)
	}
	out := make([]FilteredCandidate, 0, total)
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

func sortFilteredCandidates(candidates []Candidate, out []FilteredCandidate) {
	sort.Slice(out, func(i, j int) bool {
		return lessFilteredCandidate(candidates, out[i], out[j])
	})
}

func lessFilteredCandidate(candidates []Candidate, left FilteredCandidate, right FilteredCandidate) bool {
	if left.Score != right.Score {
		return left.Score > right.Score
	}

	leftCand := candidates[int(left.Index)]
	rightCand := candidates[int(right.Index)]
	if leftCand.Key != rightCand.Key {
		return leftCand.Key < rightCand.Key
	}
	return leftCand.ID < rightCand.ID
}

func MergeFilteredCandidates(candidates []Candidate, left []FilteredCandidate, right []FilteredCandidate) []FilteredCandidate {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}

	out := make([]FilteredCandidate, 0, len(left)+len(right))
	i, j := 0, 0
	for i < len(left) && j < len(right) {
		if lessFilteredCandidate(candidates, left[i], right[j]) {
			out = append(out, left[i])
			i++
		} else {
			out = append(out, right[j])
			j++
		}
	}
	if i < len(left) {
		out = append(out, left[i:]...)
	}
	if j < len(right) {
		out = append(out, right[j:]...)
	}

	return out
}

func scoreCandidate(cand *Candidate, index int32, qRaw []rune, qLower []rune, caseSensitive bool) (FilteredCandidate, bool) {
	keyScore, _, keyOK := fuzzyScore(cand.Key, qRaw, qLower, caseSensitive)
	textScore, textSpan, textOK := fuzzyScore(cand.Text, qRaw, qLower, caseSensitive)
	pathScore, pathSpan, pathOK := fuzzyScore(cand.File, qRaw, qLower, caseSensitive)

	queryLen := nonSpaceRuneCount(qLower)
	if textOK && rejectLooseFuzzyMatch(textScore, textSpan, queryLen) {
		textOK = false
	}
	if pathOK && rejectLooseFuzzyMatch(pathScore, pathSpan, queryLen) {
		pathOK = false
	}

	if !keyOK && !textOK && !pathOK {
		return FilteredCandidate{}, false
	}

	score := int32(-1 << 20)
	if keyOK {
		score = maxInt32(score, int32(3000+keyScore*3))
	}
	if textOK {
		score = maxInt32(score, int32(1800+textScore*2-60))
	}
	if pathOK {
		score = maxInt32(score, int32(1200+pathScore-120))
	}
	if keyOK {
		score += int32(candidateSemanticScore(cand))
	}

	if keyOK && textOK {
		score += 80
	}

	item := FilteredCandidate{Index: index, Score: score}
	if pathOK && !textOK && (!keyOK || keyLooksLikeFilename(cand)) {
		item.OpenLine = 1
		item.OpenCol = 1
	}

	return item, true
}

func rejectLooseFuzzyMatch(score int, span int, queryLen int) bool {
	if queryLen <= 1 || span <= 0 {
		return false
	}
	if span <= queryLen*5 {
		return false
	}
	return score < queryLen*4
}

func keyLooksLikeFilename(cand *Candidate) bool {
	if cand == nil {
		return false
	}
	base := fileBaseWithoutExt(cand.File)
	if base == "" || cand.Key == "" {
		return false
	}
	return strings.EqualFold(base, cand.Key)
}

func maxInt32(a int32, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
