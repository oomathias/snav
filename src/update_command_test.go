package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunUpdateCommandUsesExecutableDir(t *testing.T) {
	exeDir := filepath.Join(t.TempDir(), "bin")
	executable := filepath.Join(exeDir, "snav")

	var gotInstallDir string
	err := runUpdateCommand(
		context.Background(),
		nil,
		&bytes.Buffer{},
		&bytes.Buffer{},
		executable,
		func(_ context.Context, installDir string, stdout io.Writer, stderr io.Writer) error {
			gotInstallDir = installDir
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runUpdateCommand returned error: %v", err)
	}
	if gotInstallDir != exeDir {
		t.Fatalf("InstallDir = %q, want %q", gotInstallDir, exeDir)
	}
}

func TestRunUpdateCommandUsesSymlinkTargetDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink setup differs on Windows")
	}

	root := t.TempDir()
	targetDir := filepath.Join(root, "real-bin")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll targetDir: %v", err)
	}
	targetPath := filepath.Join(targetDir, "snav")
	if err := os.WriteFile(targetPath, []byte(""), 0o755); err != nil {
		t.Fatalf("WriteFile targetPath: %v", err)
	}

	linkDir := filepath.Join(root, "link-bin")
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatalf("MkdirAll linkDir: %v", err)
	}
	linkPath := filepath.Join(linkDir, "snav")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	var gotInstallDir string
	err := runUpdateCommand(
		context.Background(),
		nil,
		&bytes.Buffer{},
		&bytes.Buffer{},
		linkPath,
		func(_ context.Context, installDir string, stdout io.Writer, stderr io.Writer) error {
			gotInstallDir = installDir
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runUpdateCommand returned error: %v", err)
	}
	wantDir, err := filepath.EvalSymlinks(targetDir)
	if err != nil {
		t.Fatalf("EvalSymlinks targetDir: %v", err)
	}
	if gotInstallDir != wantDir {
		t.Fatalf("InstallDir = %q, want %q", gotInstallDir, wantDir)
	}
}

func TestRunUpdateCommandHelpSkipsInstaller(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	called := false

	err := runUpdateCommand(
		context.Background(),
		[]string{"--help"},
		&stdout,
		&stderr,
		"/tmp/snav",
		func(_ context.Context, installDir string, stdout io.Writer, stderr io.Writer) error {
			called = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runUpdateCommand returned error: %v", err)
	}
	if called {
		t.Fatalf("expected installer not to run for --help")
	}
	if !strings.Contains(stderr.String(), "SNAV_VERSION=latest") {
		t.Fatalf("help output missing installer description:\n%s", stderr.String())
	}
}

func TestRunUpdateCommandRejectsPositionalArgs(t *testing.T) {
	err := runUpdateCommand(
		context.Background(),
		[]string{"extra"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		"/tmp/snav",
		func(_ context.Context, installDir string, stdout io.Writer, stderr io.Writer) error {
			t.Fatalf("installer should not run when arguments are invalid")
			return nil
		},
	)
	if err == nil {
		t.Fatalf("expected positional args to fail")
	}
	if !strings.Contains(err.Error(), "does not accept positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}
