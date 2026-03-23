package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const updateInstallScriptURL = "https://raw.githubusercontent.com/oomathias/snav/main/install"

type updateInstallRunner func(context.Context, string, io.Writer, io.Writer) error

func maybeHandleCommand(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) (bool, error) {
	if len(args) == 0 || args[0] != "update" {
		return false, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return true, fmt.Errorf("resolve executable: %w", err)
	}
	return true, runUpdateCommand(ctx, args[1:], stdout, stderr, executable, runUpdateInstaller)
}

func runUpdateCommand(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	executable string,
	runInstaller updateInstallRunner,
) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printUpdateUsage(stderr, executable)
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("update does not accept positional arguments")
	}

	installDir, err := resolveExecutableInstallDir(executable)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "updating snav in %s\n", installDir); err != nil {
		return fmt.Errorf("write update status: %w", err)
	}

	return runInstaller(ctx, installDir, stdout, stderr)
}

func printUpdateUsage(out io.Writer, program string) {
	name := filepath.Base(program)
	if _, err := fmt.Fprintf(out, "Usage of %s update:\n", name); err != nil {
		fatalf("write usage: %v", err)
	}
	if _, err := fmt.Fprintf(out, "  %s update\n\n", name); err != nil {
		fatalf("write usage: %v", err)
	}
	if _, err := fmt.Fprintln(out, "Reinstall the latest snav release into the current executable directory."); err != nil {
		fatalf("write usage: %v", err)
	}
	if _, err := fmt.Fprintln(out, "The command runs the published install script with SNAV_VERSION=latest."); err != nil {
		fatalf("write usage: %v", err)
	}
}

func resolveExecutableInstallDir(executable string) (string, error) {
	if strings.TrimSpace(executable) == "" {
		return "", fmt.Errorf("resolve executable: empty path")
	}

	abs, err := filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}

	return filepath.Dir(abs), nil
}

func runUpdateInstaller(ctx context.Context, installDir string, stdout io.Writer, stderr io.Writer) error {
	curlPath, err := exec.LookPath("curl")
	if err != nil {
		return fmt.Errorf("curl is required to download updates: %w", err)
	}

	bashPath, err := exec.LookPath("bash")
	if err != nil {
		return fmt.Errorf("bash is required to run updates: %w", err)
	}

	scriptFile, err := os.CreateTemp("", "snav-install-*.sh")
	if err != nil {
		return fmt.Errorf("create installer temp file: %w", err)
	}
	scriptPath := scriptFile.Name()
	if err := scriptFile.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("close installer temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(scriptPath)
	}()

	curlCmd := exec.CommandContext(ctx,
		curlPath,
		"--fail",
		"--show-error",
		"--silent",
		"--location",
		"--proto",
		"=https",
		"--tlsv1.2",
		updateInstallScriptURL,
		"-o",
		scriptPath,
	)
	curlCmd.Stdout = io.Discard
	curlCmd.Stderr = stderr
	if err := curlCmd.Run(); err != nil {
		return fmt.Errorf("download install script: %w", err)
	}

	bashCmd := exec.CommandContext(ctx, bashPath, scriptPath)
	bashCmd.Env = append(os.Environ(),
		"SNAV_INSTALL_DIR="+installDir,
		"SNAV_VERSION=latest",
	)
	bashCmd.Stdin = os.Stdin
	bashCmd.Stdout = stdout
	bashCmd.Stderr = stderr
	if err := bashCmd.Run(); err != nil {
		return fmt.Errorf("run install script: %w", err)
	}

	return nil
}
