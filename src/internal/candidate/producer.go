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
		args := rgArgs(cfg, pattern)

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
				LangID: lang.Detect(file),
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

func rgArgs(cfg ProducerConfig, pattern string) []string {
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

	args = append(args, pattern, ".")
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
