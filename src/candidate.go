package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode"
)

const defaultRGPattern = `^\s*(?:(?:export|default|async|public|private|protected|internal|abstract|final|sealed|partial|static|inline)\s+)*(?:func|function|type|var|const|class|interface|enum|def|fn|struct|impl|trait|module|mod|let|protocol|extension|namespace)\b`

type Candidate struct {
	ID     int
	File   string
	Line   int
	Col    int
	Text   string
	Key    string
	LangID LangID
}

type ProducerConfig struct {
	Root         string
	Pattern      string
	Excludes     []string
	NoIgnore     bool
	ExcludeTests bool
}

var testExcludeGlobs = []string{
	"test/**",
	"tests/**",
	"__tests__/**",
	"spec/**",
	"specs/**",
	"**/test/**",
	"**/tests/**",
	"**/__tests__/**",
	"**/spec/**",
	"**/specs/**",
	"*_test.*",
	"*_spec.*",
	"*.test.*",
	"*.spec.*",
	"test_*.py",
	"**/*_test.*",
	"**/*_spec.*",
	"**/*.test.*",
	"**/*.spec.*",
	"**/test_*.py",
}

func StartProducer(ctx context.Context, cfg ProducerConfig) (<-chan Candidate, <-chan error) {
	out := make(chan Candidate, 4096)
	done := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(done)

		args := []string{
			"--vimgrep",
			"--null",
			"--trim",
			"--color", "never",
			"--no-heading",
			"--smart-case",
		}
		if cfg.NoIgnore {
			args = append(args, "--no-ignore")
		}
		for _, glob := range cfg.Excludes {
			args = append(args, "--glob", "!"+glob)
		}
		if cfg.ExcludeTests {
			for _, glob := range testExcludeGlobs {
				args = append(args, "--glob", "!"+glob)
			}
		}

		pattern := strings.TrimSpace(cfg.Pattern)
		if pattern == "" {
			pattern = defaultRGPattern
		}

		args = append(args, pattern, ".")

		cmd := exec.CommandContext(ctx, "rg", args...)
		cmd.Dir = cfg.Root

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			done <- fmt.Errorf("open rg stdout: %w", err)
			return
		}

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			done <- fmt.Errorf("start rg: %w", err)
			return
		}

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 128*1024), 8*1024*1024)

		id := 0
		for scanner.Scan() {
			file, line, col, text, ok := parseRGVimgrepLine(scanner.Bytes())
			if !ok {
				continue
			}
			id++
			cand := Candidate{
				ID:     id,
				File:   filepath.Clean(file),
				Line:   line,
				Col:    col,
				Text:   text,
				Key:    ExtractKey(text, file),
				LangID: DetectLanguage(file),
			}

			select {
			case out <- cand:
			case <-ctx.Done():
				done <- ctx.Err()
				return
			}
		}

		if err := scanner.Err(); err != nil {
			done <- fmt.Errorf("read rg output: %w", err)
			return
		}

		if err := cmd.Wait(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
				done <- nil
				return
			}
			msg := strings.TrimSpace(stderr.String())
			if msg != "" {
				done <- fmt.Errorf("rg failed: %s", msg)
				return
			}
			done <- fmt.Errorf("rg failed: %w", err)
			return
		}

		done <- nil
	}()

	return out, done
}

func parseRGVimgrepLine(raw []byte) (file string, lineNo int, colNo int, text string, ok bool) {
	nul := bytes.IndexByte(raw, 0)
	if nul <= 0 || nul >= len(raw)-1 {
		return "", 0, 0, "", false
	}

	file = string(raw[:nul])
	rest := raw[nul+1:]

	sep1 := bytes.IndexByte(rest, ':')
	if sep1 <= 0 {
		return "", 0, 0, "", false
	}
	parsedLine, ok := parsePositiveIntBytes(rest[:sep1])
	if !ok {
		return "", 0, 0, "", false
	}
	rest = rest[sep1+1:]

	sep2 := bytes.IndexByte(rest, ':')
	if sep2 <= 0 {
		return "", 0, 0, "", false
	}
	parsedCol, ok := parsePositiveIntBytes(rest[:sep2])
	if !ok {
		return "", 0, 0, "", false
	}

	lineNo = parsedLine
	colNo = parsedCol
	text = strings.TrimRight(string(rest[sep2+1:]), "\r")
	return file, lineNo, colNo, text, true
}

func parsePositiveIntBytes(raw []byte) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	v := 0
	for _, b := range raw {
		if b < '0' || b > '9' {
			return 0, false
		}
		v = v*10 + int(b-'0')
	}
	if v <= 0 {
		return 0, false
	}
	return v, true
}

var keyRegexes = []*regexp.Regexp{
	regexp.MustCompile(`^\s*(?:export\s+)?(?:inline\s+)?namespace\s+([A-Za-z_][A-Za-z0-9_]*(?:(?:::|\.)[A-Za-z_][A-Za-z0-9_]*)*)\b`),
	regexp.MustCompile(`^\s*(?:(?:export|default|async|public|private|protected|internal|abstract|final|sealed|partial|static)\s+)*(?:function|class|interface|type|enum|record)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`),
	regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	regexp.MustCompile(`^\s*func\s*(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`),
	regexp.MustCompile(`^\s*(?:type|var|const)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`^\s*(?:pub\s+)?(?:fn|struct|enum|trait|mod|type|const|static)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`^\s*(?:async\s+def|def|class)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`^\s*(?:interface|class|enum|record)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`^\s*(?:fun|val|var|object|class|interface)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*:=`),
	regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*:`),
}

var firstIdentifier = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

func ExtractKey(text string, file string) string {
	for _, re := range keyRegexes {
		m := re.FindStringSubmatch(text)
		if len(m) > 1 {
			for _, group := range m[1:] {
				if group != "" {
					return group
				}
			}
		}
	}

	if ident := firstIdentifier.FindString(text); ident != "" && !matcherStopWords[strings.ToLower(ident)] {
		return ident
	}

	base := filepath.Base(file)
	ext := filepath.Ext(base)
	base = strings.TrimSuffix(base, ext)
	if base == "" {
		return file
	}
	return base
}

type filteredCandidate struct {
	Index int32
	Score int32
}

var filterParallelThreshold = 20_000
var filterMinChunkSize = 4_096

func filterCandidates(candidates []Candidate, query string) []filteredCandidate {
	q := trimRunes(query)
	return filterCandidatesWithQueryRunes(candidates, q, lowerRunes(q))
}

func filterCandidatesWithRunes(candidates []Candidate, q []rune) []filteredCandidate {
	return filterCandidatesWithQueryRunes(candidates, nil, q)
}

func filterCandidatesSubsetWithQueryRunes(candidates []Candidate, subset []filteredCandidate, qRaw []rune, qLower []rune) []filteredCandidate {
	return filterCandidatesCore(candidates, subset, qRaw, qLower)
}

func filterCandidatesRangeWithQueryRunes(candidates []Candidate, start int, end int, qRaw []rune, qLower []rune) []filteredCandidate {
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

	var out []filteredCandidate
	if workers <= 1 {
		out = make([]filteredCandidate, 0, max(1, n/4))
		for i := start; i < end; i++ {
			item, ok := scoreCandidate(&candidates[i], int32(i), qRaw, qLower, caseSensitive)
			if !ok {
				continue
			}
			out = append(out, item)
		}
	} else {
		parts := make([][]filteredCandidate, workers)
		var wg sync.WaitGroup
		for worker := 0; worker < workers; worker++ {
			chunkStart := start + worker*n/workers
			chunkEnd := start + (worker+1)*n/workers
			wg.Add(1)
			go func(slot int, chunkStart int, chunkEnd int) {
				defer wg.Done()
				local := make([]filteredCandidate, 0, max(1, (chunkEnd-chunkStart)/4))
				for i := chunkStart; i < chunkEnd; i++ {
					item, ok := scoreCandidate(&candidates[i], int32(i), qRaw, qLower, caseSensitive)
					if !ok {
						continue
					}
					local = append(local, item)
				}
				parts[slot] = local
			}(worker, chunkStart, chunkEnd)
		}
		wg.Wait()
		out = flattenFilteredParts(parts)
	}

	sortFilteredCandidates(candidates, out)
	return out
}

func filterCandidatesWithQueryRunes(candidates []Candidate, qRaw []rune, qLower []rune) []filteredCandidate {
	return filterCandidatesCore(candidates, nil, qRaw, qLower)
}

func filterCandidatesCore(candidates []Candidate, subset []filteredCandidate, qRaw []rune, qLower []rune) []filteredCandidate {
	if len(qLower) == 0 {
		out := make([]filteredCandidate, len(candidates))
		for i := range candidates {
			out[i] = filteredCandidate{Index: int32(i)}
		}
		return out
	}
	if subset != nil && len(subset) == 0 {
		return nil
	}

	caseSensitive := len(qRaw) == len(qLower)
	n := len(candidates)
	if subset != nil {
		n = len(subset)
	}

	workers := filterWorkerCount(n)
	var out []filteredCandidate
	if workers <= 1 {
		if subset == nil {
			out = filterCandidatesSerial(candidates, qRaw, qLower, caseSensitive)
		} else {
			out = filterCandidatesSubsetSerial(candidates, subset, qRaw, qLower, caseSensitive)
		}
	} else {
		if subset == nil {
			out = filterCandidatesParallel(candidates, qRaw, qLower, caseSensitive, workers)
		} else {
			out = filterCandidatesSubsetParallel(candidates, subset, qRaw, qLower, caseSensitive, workers)
		}
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
	if workers < 2 {
		return 1
	}

	return workers
}

func filterCandidatesSerial(candidates []Candidate, qRaw []rune, qLower []rune, caseSensitive bool) []filteredCandidate {
	out := make([]filteredCandidate, 0, len(candidates)/4)
	for i := range candidates {
		item, ok := scoreCandidate(&candidates[i], int32(i), qRaw, qLower, caseSensitive)
		if !ok {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterCandidatesSubsetSerial(candidates []Candidate, subset []filteredCandidate, qRaw []rune, qLower []rune, caseSensitive bool) []filteredCandidate {
	out := make([]filteredCandidate, 0, len(subset)/2)
	for _, base := range subset {
		idx := int(base.Index)
		if idx < 0 || idx >= len(candidates) {
			continue
		}
		item, ok := scoreCandidate(&candidates[idx], base.Index, qRaw, qLower, caseSensitive)
		if !ok {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterCandidatesParallel(candidates []Candidate, qRaw []rune, qLower []rune, caseSensitive bool, workers int) []filteredCandidate {
	parts := make([][]filteredCandidate, workers)
	var wg sync.WaitGroup

	for worker := 0; worker < workers; worker++ {
		start := worker * len(candidates) / workers
		end := (worker + 1) * len(candidates) / workers
		wg.Add(1)
		go func(slot int, start int, end int) {
			defer wg.Done()
			local := make([]filteredCandidate, 0, max(1, (end-start)/4))
			for i := start; i < end; i++ {
				item, ok := scoreCandidate(&candidates[i], int32(i), qRaw, qLower, caseSensitive)
				if !ok {
					continue
				}
				local = append(local, item)
			}
			parts[slot] = local
		}(worker, start, end)
	}

	wg.Wait()
	return flattenFilteredParts(parts)
}

func filterCandidatesSubsetParallel(candidates []Candidate, subset []filteredCandidate, qRaw []rune, qLower []rune, caseSensitive bool, workers int) []filteredCandidate {
	parts := make([][]filteredCandidate, workers)
	var wg sync.WaitGroup

	for worker := 0; worker < workers; worker++ {
		start := worker * len(subset) / workers
		end := (worker + 1) * len(subset) / workers
		wg.Add(1)
		go func(slot int, start int, end int) {
			defer wg.Done()
			local := make([]filteredCandidate, 0, max(1, (end-start)/2))
			for i := start; i < end; i++ {
				idx := int(subset[i].Index)
				if idx < 0 || idx >= len(candidates) {
					continue
				}
				item, ok := scoreCandidate(&candidates[idx], subset[i].Index, qRaw, qLower, caseSensitive)
				if !ok {
					continue
				}
				local = append(local, item)
			}
			parts[slot] = local
		}(worker, start, end)
	}

	wg.Wait()
	return flattenFilteredParts(parts)
}

func flattenFilteredParts(parts [][]filteredCandidate) []filteredCandidate {
	total := 0
	for _, part := range parts {
		total += len(part)
	}
	out := make([]filteredCandidate, 0, total)
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

func sortFilteredCandidates(candidates []Candidate, out []filteredCandidate) {
	sort.Slice(out, func(i, j int) bool {
		return lessFilteredCandidate(candidates, out[i], out[j])
	})
}

func lessFilteredCandidate(candidates []Candidate, left filteredCandidate, right filteredCandidate) bool {
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

func mergeFilteredCandidates(candidates []Candidate, left []filteredCandidate, right []filteredCandidate) []filteredCandidate {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}

	out := make([]filteredCandidate, 0, len(left)+len(right))
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

func scoreCandidate(cand *Candidate, index int32, qRaw []rune, qLower []rune, caseSensitive bool) (filteredCandidate, bool) {
	keyScore, keyOK := fuzzyScore(cand.Key, qRaw, qLower, caseSensitive)
	textScore, textOK := fuzzyScore(cand.Text, qRaw, qLower, caseSensitive)
	pathScore, pathOK := fuzzyScore(cand.File, qRaw, qLower, caseSensitive)

	if !keyOK && !textOK && !pathOK {
		return filteredCandidate{}, false
	}

	score := int32(-1 << 20)
	if keyOK {
		score = max(score, int32(3000+keyScore*3))
	}
	if textOK {
		score = max(score, int32(1800+textScore*2-60))
	}
	if pathOK {
		score = max(score, int32(1200+pathScore-120))
	}

	if keyOK && textOK {
		score += 80
	}

	return filteredCandidate{Index: index, Score: score}, true
}

func fuzzyScore(text string, queryRaw []rune, queryLower []rune, caseSensitive bool) (int, bool) {
	if len(queryLower) == 0 {
		return 0, true
	}

	qi := 0
	last := -2
	score := 0
	runeIdx := 0
	var prev rune
	hasPrev := false
	caseMatches := 0

	for _, raw := range text {
		r := lowerRuneFast(raw)

		if qi < len(queryLower) && r == queryLower[qi] {
			bonus := 10
			if runeIdx == 0 || (hasPrev && isBoundaryRune(prev)) {
				bonus += 8
			}
			if last+1 == runeIdx {
				bonus += 6
			}
			if caseSensitive && raw == queryRaw[qi] {
				bonus += 4
				caseMatches++
			}

			score += bonus
			last = runeIdx
			qi++
		}

		prev = r
		hasPrev = true
		runeIdx++
	}

	if qi != len(queryLower) {
		return 0, false
	}

	if runeIdx > len(queryLower) {
		score -= runeIdx - len(queryLower)
	}
	if runeIdx < 40 {
		score += 40 - runeIdx
	}
	if caseMatches > 0 {
		score += caseMatches * 3
	}

	return score, true
}

func fuzzyPositions(text string, query string) []int {
	return fuzzyPositionsRunes(text, lowerTrimRunes(query))
}

func fuzzyPositionsRunes(text string, queryLower []rune) []int {
	if len(queryLower) == 0 {
		return nil
	}

	out := make([]int, 0, len(queryLower))
	qi := 0
	idx := 0
	for _, raw := range text {
		if qi >= len(queryLower) {
			break
		}
		if lowerRuneFast(raw) == queryLower[qi] {
			out = append(out, idx)
			qi++
		}
		idx++
	}
	if qi != len(queryLower) {
		return nil
	}
	return out
}

func lowerTrimRunes(s string) []rune {
	return lowerRunes(trimRunes(s))
}

func trimRunes(s string) []rune {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return []rune(s)
}

func lowerRunes(r []rune) []rune {
	if len(r) == 0 {
		return nil
	}
	out := make([]rune, len(r))
	for i := range r {
		out[i] = lowerRuneFast(r[i])
	}
	return out
}

func lowerRuneFast(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	if r <= unicode.MaxASCII {
		return r
	}
	return unicode.ToLower(r)
}

var matcherStopWords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "return": true,
	"case": true, "break": true, "continue": true, "default": true,
	"func": true, "type": true, "const": true, "var": true,
	"class": true, "interface": true, "enum": true,
	"namespace": true,
	"public":    true, "private": true, "protected": true, "internal": true,
	"abstract": true, "final": true, "sealed": true, "partial": true,
	"static": true, "inline": true,
	"def": true, "fn": true,
}

func isBoundaryRune(r rune) bool {
	switch r {
	case '_', '-', '/', '.', ':':
		return true
	}
	if r <= unicode.MaxASCII {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
