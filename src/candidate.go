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
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const defaultRGPattern = `^\s*(?:export\s+)?(?:async\s+)?(?:func|type|var|const|class|interface|enum|def|fn|struct|impl|trait|module|mod|let|protocol|extension)\b`

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
	Root     string
	Pattern  string
	Excludes []string
	NoIgnore bool
	NoTest   bool
}

var noTestExcludeGlobs = []string{
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
	"**/*test*",
	"**/*spec*",
}

func StartProducer(ctx context.Context, cfg ProducerConfig) (<-chan Candidate, <-chan error) {
	out := make(chan Candidate, 4096)
	done := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(done)

		args := []string{
			"--vimgrep",
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
		if cfg.NoTest {
			for _, glob := range noTestExcludeGlobs {
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
			file, line, col, text, ok := parseRGLine(scanner.Text())
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

func parseRGLine(line string) (file string, lineNo int, colNo int, text string, ok bool) {
	parts := strings.SplitN(line, ":", 4)
	if len(parts) != 4 {
		return "", 0, 0, "", false
	}

	lineNo, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, 0, "", false
	}
	colNo, err = strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, 0, "", false
	}
	return parts[0], lineNo, colNo, parts[3], true
}

var keyRegexes = []*regexp.Regexp{
	regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?(?:function|class|interface|type|enum)\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
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
			for i := 1; i < len(m); i++ {
				if m[i] != "" {
					return m[i]
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
	Index int
	Score int
}

func filterCandidates(candidates []Candidate, query string) []filteredCandidate {
	return filterCandidatesWithRunes(candidates, lowerTrimRunes(query))
}

func filterCandidatesWithRunes(candidates []Candidate, q []rune) []filteredCandidate {
	if len(q) == 0 {
		out := make([]filteredCandidate, len(candidates))
		for i := range candidates {
			out[i] = filteredCandidate{Index: i}
		}
		return out
	}

	out := make([]filteredCandidate, 0, len(candidates)/4)
	for i := range candidates {
		cand := candidates[i]

		keyScore, keyOK := fuzzyScore(cand.Key, q)
		textScore, textOK := fuzzyScore(cand.Text, q)
		pathScore, pathOK := fuzzyScore(cand.File, q)

		if !keyOK && !textOK && !pathOK {
			continue
		}

		score := -1 << 20
		if keyOK {
			score = max(score, 3000+keyScore*3)
		}
		if textOK {
			score = max(score, 1800+textScore*2-60)
		}
		if pathOK {
			score = max(score, 1200+pathScore-120)
		}

		if keyOK && textOK {
			score += 80
		}

		out = append(out, filteredCandidate{Index: i, Score: score})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			left := candidates[out[i].Index]
			right := candidates[out[j].Index]
			if left.Key == right.Key {
				return left.ID < right.ID
			}
			return left.Key < right.Key
		}
		return out[i].Score > out[j].Score
	})

	return out
}

func fuzzyScore(text string, queryLower []rune) (int, bool) {
	if len(queryLower) == 0 {
		return 0, true
	}

	qi := 0
	last := -2
	score := 0
	runeIdx := 0
	var prev rune
	hasPrev := false

	for _, raw := range text {
		r := unicode.ToLower(raw)

		if qi < len(queryLower) && r == queryLower[qi] {
			bonus := 10
			if runeIdx == 0 || (hasPrev && isBoundaryRune(prev)) {
				bonus += 8
			}
			if last+1 == runeIdx {
				bonus += 6
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
		if unicode.ToLower(raw) == queryLower[qi] {
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
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	r := []rune(s)
	for i := range r {
		r[i] = unicode.ToLower(r[i])
	}
	return r
}

var matcherStopWords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "return": true,
	"case": true, "break": true, "continue": true, "default": true,
	"func": true, "type": true, "const": true, "var": true,
	"class": true, "interface": true, "enum": true,
	"def": true, "fn": true,
}

func isBoundaryRune(r rune) bool {
	if r == '_' || r == '-' || r == '/' || r == '.' || r == ':' {
		return true
	}
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
