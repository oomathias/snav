package main

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
)

func openLocation(path string, line int, col int, editorCmd string) error {
	target := fmt.Sprintf("%s:%d:%d", path, line, col)

	if strings.TrimSpace(editorCmd) != "" {
		name, args, err := buildEditorCommand(editorCmd, path, line, col, target)
		if err != nil {
			return err
		}
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("editor command not found: %s", name)
		}
		return exec.Command(name, args...).Start()
	}

	if _, err := exec.LookPath("zed"); err == nil {
		cmd := exec.Command("zed", target)
		return cmd.Start()
	}

	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("open"); err == nil {
			return exec.Command("open", path).Start()
		}
		return fmt.Errorf("zed and open are unavailable")
	case "linux":
		if _, err := exec.LookPath("xdg-open"); err == nil {
			return exec.Command("xdg-open", path).Start()
		}
		return fmt.Errorf("zed and xdg-open are unavailable")
	case "windows":
		if _, err := exec.LookPath("explorer.exe"); err == nil {
			return exec.Command("explorer.exe", path).Start()
		}
		if _, err := exec.LookPath("cmd"); err == nil {
			return exec.Command("cmd", "/C", "start", "", path).Start()
		}
		return fmt.Errorf("zed and explorer are unavailable")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func buildEditorCommand(template string, file string, line int, col int, target string) (string, []string, error) {
	parts, err := splitCommandLine(strings.TrimSpace(template))
	if err != nil {
		return "", nil, err
	}
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("editor command is empty")
	}

	repl := map[string]string{
		"{file}":   file,
		"{line}":   fmt.Sprintf("%d", line),
		"{col}":    fmt.Sprintf("%d", col),
		"{target}": target,
	}

	for i := range parts {
		for k, v := range repl {
			parts[i] = strings.ReplaceAll(parts[i], k, v)
		}
	}

	return parts[0], parts[1:], nil
}

func splitCommandLine(input string) ([]string, error) {
	var parts []string
	var current strings.Builder

	tokenActive := false
	inSingle := false
	inDouble := false

	flush := func() {
		if !tokenActive {
			return
		}
		parts = append(parts, current.String())
		current.Reset()
		tokenActive = false
	}

	for _, r := range input {
		switch r {
		case '\'':
			if inDouble {
				current.WriteRune(r)
				tokenActive = true
				continue
			}
			inSingle = !inSingle
			tokenActive = true
		case '"':
			if inSingle {
				current.WriteRune(r)
				tokenActive = true
				continue
			}
			inDouble = !inDouble
			tokenActive = true
		case ' ', '\t', '\n', '\r':
			if inSingle || inDouble {
				current.WriteRune(r)
				tokenActive = true
				continue
			}
			flush()
		default:
			current.WriteRune(r)
			tokenActive = true
		}
	}

	if inSingle || inDouble {
		return nil, fmt.Errorf("editor command has unclosed quote")
	}

	flush()
	return parts, nil
}

func copyToClipboard(s string) error {
	switch runtime.GOOS {
	case "darwin":
		return pipeStringToCommand(s, "pbcopy")
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			return pipeStringToCommand(s, "wl-copy")
		}
		if _, err := exec.LookPath("xclip"); err == nil {
			return pipeStringToCommand(s, "xclip", "-selection", "clipboard")
		}
		if _, err := exec.LookPath("xsel"); err == nil {
			return pipeStringToCommand(s, "xsel", "--clipboard", "--input")
		}
		return fmt.Errorf("no clipboard utility found (install wl-copy, xclip, or xsel)")
	case "windows":
		return pipeStringToCommand(s, "clip")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func pipeStringToCommand(input string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := io.WriteString(in, input); err != nil {
		_ = in.Close()
		_ = cmd.Wait()
		return err
	}
	if err := in.Close(); err != nil {
		_ = cmd.Wait()
		return err
	}
	return cmd.Wait()
}
