package candidate

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"snav/internal/lang"
	"strings"
)

type candidateLocation struct {
	file string
	line int
	col  int
}

func StartProducer(ctx context.Context, cfg ProducerConfig) (<-chan Candidate, <-chan error) {
	out := make(chan Candidate, 4096)
	done := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(done)

		pattern := strings.TrimSpace(cfg.Pattern)
		if pattern == "" {
			pattern = DefaultRGPattern
		}
		id := 0
		seen := make(map[candidateLocation]struct{}, 4096)
		emitMatch := func(file string, line int, col int, text string) error {
			cleanFile := filepath.Clean(file)
			loc := candidateLocation{file: cleanFile, line: line, col: col}
			if _, exists := seen[loc]; exists {
				return nil
			}
			seen[loc] = struct{}{}

			id++
			cand := Candidate{
				ID:            id,
				File:          cleanFile,
				Line:          line,
				Col:           col,
				Text:          text,
				Key:           ExtractKey(text, cleanFile),
				LangID:        lang.Detect(cleanFile),
				SemanticScore: computeSemanticScore(text),
			}

			select {
			case out <- cand:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if err := runRGPass(ctx, cfg.Root, rgArgs(cfg, pattern), emitMatch); err != nil {
			done <- fmt.Errorf("search declarations: %w", err)
			return
		}
		if shouldIncludeConfigPass(pattern) {
			if err := runRGPass(ctx, cfg.Root, rgConfigArgs(cfg), emitMatch); err != nil {
				done <- fmt.Errorf("search config entries: %w", err)
				return
			}
		}

		done <- nil
	}()

	return out, done
}

func runRGPass(ctx context.Context, root string, args []string, onMatch func(file string, line int, col int, text string) error) error {
	cmd := exec.CommandContext(ctx, "rg", args...)
	cmd.Dir = root

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open rg stdout: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start rg: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 128*1024), 8*1024*1024)

	for scanner.Scan() {
		file, line, col, text, ok := parseRGVimgrepLine(scanner.Bytes())
		if !ok {
			continue
		}
		if err := onMatch(file, line, col, text); err != nil {
			_ = cmd.Wait()
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read rg output: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil
		}
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("rg failed: %s", msg)
		}
		return fmt.Errorf("rg failed: %w", err)
	}

	return nil
}

func shouldIncludeConfigPass(pattern string) bool {
	return strings.TrimSpace(pattern) == DefaultRGPattern
}

func rgArgs(cfg ProducerConfig, pattern string) []string {
	args := rgBaseArgs(cfg)
	args = append(args, pattern, ".")
	return args
}

func rgConfigArgs(cfg ProducerConfig) []string {
	args := rgBaseArgs(cfg)
	for _, glob := range configIncludeGlobs {
		args = append(args, "--glob", glob)
	}
	args = append(args, DefaultRGConfigPattern, ".")
	return args
}

func rgBaseArgs(cfg ProducerConfig) []string {
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
	return args
}

func parseRGVimgrepLine(raw []byte) (file string, lineNo int, colNo int, text string, ok bool) {
	nul := bytes.IndexByte(raw, 0)
	if nul <= 0 || nul >= len(raw)-1 {
		return "", 0, 0, "", false
	}

	file = string(raw[:nul])
	rest := raw[nul+1:]

	parsedLine, rest, ok := parsePositiveIntField(rest)
	if !ok {
		return "", 0, 0, "", false
	}
	parsedCol, rest, ok := parsePositiveIntField(rest)
	if !ok {
		return "", 0, 0, "", false
	}

	lineNo = parsedLine
	colNo = parsedCol
	text = strings.TrimRight(string(rest), "\r")
	return file, lineNo, colNo, text, true
}

func parsePositiveIntField(raw []byte) (int, []byte, bool) {
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
