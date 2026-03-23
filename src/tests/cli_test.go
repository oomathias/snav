package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return runCLIWithEnv(t, nil, args...)
}

func runCLIWithEnv(t *testing.T, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "../"}, args...)...)
	if env != nil {
		cmd.Env = mergeEnv(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func mergeEnv(base []string, overrides ...string) []string {
	env := make([]string, 0, len(base)+len(overrides))
	seen := make(map[string]struct{}, len(overrides))
	for _, override := range overrides {
		key, _, ok := strings.Cut(override, "=")
		if !ok {
			continue
		}
		seen[key] = struct{}{}
	}
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, skip := seen[key]; skip {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, overrides...)
	return env
}

func TestExternalHelpFlag(t *testing.T) {
	out, err := runCLI(t, "--help")
	if err != nil {
		t.Fatalf("expected help flag to succeed, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "Usage of") {
		t.Fatalf("help output missing usage header:\n%s", out)
	}
	if !strings.Contains(out, "  --root string") {
		t.Fatalf("help output missing long-form flag:\n%s", out)
	}
	if strings.Contains(out, "\n  -root string") {
		t.Fatalf("help output still shows single-dash flag:\n%s", out)
	}
	if !strings.Contains(out, "\nCommands:\n") {
		t.Fatalf("help output missing commands section:\n%s", out)
	}
	if !strings.Contains(out, "update  reinstall the latest release") {
		t.Fatalf("help output missing update command:\n%s", out)
	}
}

func TestExternalRejectsInvalidHighlightContext(t *testing.T) {
	out, err := runCLI(t, "--highlight-context=invalid")
	if err == nil {
		t.Fatalf("expected invalid highlight context to fail")
	}
	if !strings.Contains(out, "invalid --highlight-context") {
		t.Fatalf("unexpected error output:\n%s", out)
	}
}

func TestExternalUpdateHelpFlag(t *testing.T) {
	out, err := runCLI(t, "update", "--help")
	if err != nil {
		t.Fatalf("expected update help flag to succeed, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "SNAV_VERSION=latest") {
		t.Fatalf("update help output missing installer detail:\n%s", out)
	}
}

func TestExternalUpdateCommandUsesInstallerScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub setup differs on Windows")
	}

	stubDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(stubDir, 0o755); err != nil {
		t.Fatalf("MkdirAll stubDir: %v", err)
	}

	writeExecutable(t, filepath.Join(stubDir, "curl"), `#!/bin/sh
out=""
while [ "$#" -gt 0 ]; do
	if [ "$1" = "-o" ]; then
		out="$2"
		shift 2
		continue
	fi
	shift
done
cat > "$out" <<'EOF'
#!/bin/sh
printf '%s\n' "$SNAV_INSTALL_DIR" > "$SNAV_UPDATE_PROBE"
printf '%s\n' "$SNAV_VERSION" >> "$SNAV_UPDATE_PROBE"
EOF
`)
	writeExecutable(t, filepath.Join(stubDir, "bash"), `#!/bin/sh
exec /bin/sh "$@"
`)

	probePath := filepath.Join(t.TempDir(), "probe.txt")
	pathValue := stubDir + string(os.PathListSeparator) + os.Getenv("PATH")
	out, err := runCLIWithEnv(t, []string{
		"PATH=" + pathValue,
		"SNAV_UPDATE_PROBE=" + probePath,
	}, "update")
	if err != nil {
		t.Fatalf("expected update command to succeed, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "updating snav in ") {
		t.Fatalf("update output missing status line:\n%s", out)
	}

	data, err := os.ReadFile(probePath)
	if err != nil {
		t.Fatalf("ReadFile probePath: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("probe lines = %#v, want install dir and version", lines)
	}
	if got := lines[0]; got == "" || !filepath.IsAbs(got) {
		t.Fatalf("SNAV_INSTALL_DIR = %q, want absolute path", got)
	}
	if got := lines[1]; got != "latest" {
		t.Fatalf("SNAV_VERSION = %q, want latest", got)
	}
}

func writeExecutable(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}
