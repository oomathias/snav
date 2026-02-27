package readfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadLinesNormalized(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  []string
	}{
		{
			name: "empty file",
			in:   "",
			out:  []string{""},
		},
		{
			name: "unix newlines",
			in:   "one\ntwo\n",
			out:  []string{"one", "two", ""},
		},
		{
			name: "windows newlines",
			in:   "one\r\ntwo\r\n",
			out:  []string{"one", "two", ""},
		},
		{
			name: "standalone carriage returns preserved",
			in:   "a\rb\n\r\n",
			out:  []string{"a\rb", "", ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "input.txt")
			if err := os.WriteFile(path, []byte(tc.in), 0o644); err != nil {
				t.Fatalf("write temp file: %v", err)
			}

			got, err := ReadLinesNormalized(path)
			if err != nil {
				t.Fatalf("ReadLinesNormalized: %v", err)
			}
			if len(got) != len(tc.out) {
				t.Fatalf("lines len: got %d want %d", len(got), len(tc.out))
			}
			for i := range got {
				if got[i] != tc.out[i] {
					t.Fatalf("line %d: got %q want %q", i, got[i], tc.out[i])
				}
			}
		})
	}
}
