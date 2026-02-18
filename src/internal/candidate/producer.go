package candidate

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

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
			pattern = DefaultRGPattern
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
				LangID: detectLanguage(file),
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
