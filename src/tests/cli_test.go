package main_test

import (
	"os/exec"
	"strings"
	"testing"
)

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "../"}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestExternalHelpFlag(t *testing.T) {
	out, err := runCLI(t, "-h")
	if err != nil {
		t.Fatalf("expected help flag to succeed, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "Usage of") {
		t.Fatalf("help output missing usage header:\n%s", out)
	}
}

func TestExternalRejectsInvalidHighlightContext(t *testing.T) {
	out, err := runCLI(t, "-highlight-context=invalid")
	if err == nil {
		t.Fatalf("expected invalid highlight context to fail")
	}
	if !strings.Contains(out, "invalid -highlight-context") {
		t.Fatalf("unexpected error output:\n%s", out)
	}
}
