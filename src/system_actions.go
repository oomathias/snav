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

	commands, unavailable := openFileCommands(path)
	commands = append([][]string{{"zed", target}}, commands...)
	if found, err := runFirstAvailableCommand(commands, runCommandStart); found {
		return err
	}
	return unavailable
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
		if found, err := runFirstAvailableCommand([][]string{
			{"wl-copy"},
			{"xclip", "-selection", "clipboard"},
			{"xsel", "--clipboard", "--input"},
		}, func(name string, args []string) error {
			return pipeStringToCommand(s, name, args...)
		}); found {
			return err
		}
		return fmt.Errorf("no clipboard utility found (install wl-copy, xclip, or xsel)")
	case "windows":
		return pipeStringToCommand(s, "clip")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func openFileCommands(path string) ([][]string, error) {
	switch runtime.GOOS {
	case "darwin":
		return [][]string{{"open", path}}, fmt.Errorf("zed and open are unavailable")
	case "linux":
		return [][]string{{"xdg-open", path}}, fmt.Errorf("zed and xdg-open are unavailable")
	case "windows":
		return [][]string{{"explorer.exe", path}, {"cmd", "/C", "start", "", path}}, fmt.Errorf("zed and explorer are unavailable")
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func runCommandStart(name string, args []string) error {
	return exec.Command(name, args...).Start()
}

func runFirstAvailableCommand(candidates [][]string, run func(name string, args []string) error) (bool, error) {
	for _, candidate := range candidates {
		if len(candidate) == 0 {
			continue
		}

		name := candidate[0]
		args := candidate[1:]
		if _, err := exec.LookPath(name); err != nil {
			continue
		}

		return true, run(name, args)
	}

	return false, nil
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
