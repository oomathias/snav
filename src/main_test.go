package main

import (
	"reflect"
	"testing"

	"snav/internal/candidate"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBuildEditorCommandSupportsQuotedPathAndArgs(t *testing.T) {
	template := `"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code" -g "{target}" --reuse-window`
	name, args, err := buildEditorCommand(template, "/tmp/my file.go", 12, 4, "/tmp/my file.go:12:4")
	if err != nil {
		t.Fatalf("buildEditorCommand returned error: %v", err)
	}

	if name != "/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code" {
		t.Fatalf("name = %q", name)
	}

	wantArgs := []string{"-g", "/tmp/my file.go:12:4", "--reuse-window"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestBuildEditorCommandPreservesEmptyArgument(t *testing.T) {
	template := `cmd /C start "" "{file}"`
	name, args, err := buildEditorCommand(template, `C:\Program Files\Editor\file.go`, 8, 1, `C:\Program Files\Editor\file.go:8:1`)
	if err != nil {
		t.Fatalf("buildEditorCommand returned error: %v", err)
	}

	if name != "cmd" {
		t.Fatalf("name = %q, want cmd", name)
	}

	wantArgs := []string{"/C", "start", "", `C:\Program Files\Editor\file.go`}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestBuildEditorCommandRejectsUnclosedQuote(t *testing.T) {
	if _, _, err := buildEditorCommand(`code -g "{target}`, "file.go", 1, 1, "file.go:1:1"); err == nil {
		t.Fatalf("expected error for unclosed quote")
	}
}

func TestBuildEditorCommandKeepsBackslashes(t *testing.T) {
	name, args, err := buildEditorCommand(`C:\tools\code.exe -g {target}`, `C:\repo\file.go`, 3, 2, `C:\repo\file.go:3:2`)
	if err != nil {
		t.Fatalf("buildEditorCommand returned error: %v", err)
	}
	if name != `C:\tools\code.exe` {
		t.Fatalf("name = %q", name)
	}

	wantArgs := []string{"-g", `C:\repo\file.go:3:2`}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestShouldUseIncrementalFilter(t *testing.T) {
	if !shouldUseIncrementalFilter([]rune("handler"), []rune("hand"), 100, 100) {
		t.Fatalf("expected incremental filter to be used for growing prefix query")
	}
	if shouldUseIncrementalFilter([]rune("hand"), []rune("handler"), 100, 100) {
		t.Fatalf("did not expect incremental filter for shortened query")
	}
	if shouldUseIncrementalFilter([]rune("handler"), []rune("hand"), 101, 100) {
		t.Fatalf("did not expect incremental filter when candidate count changes")
	}
	if shouldUseIncrementalFilter([]rune("parser"), []rune("hand"), 100, 100) {
		t.Fatalf("did not expect incremental filter when query is not prefixed")
	}
}

func TestCopyRunesReuse(t *testing.T) {
	dst := []rune{'x', 'y', 'z'}
	out := copyRunesReuse(dst[:0], []rune("abc"))
	if string(out) != "abc" {
		t.Fatalf("copyRunesReuse = %q, want %q", string(out), "abc")
	}

	out = copyRunesReuse(out, nil)
	if out != nil {
		t.Fatalf("copyRunesReuse with nil source should return nil")
	}
}

func TestModelUpdateAllowsRuneJAndKInput(t *testing.T) {
	m := newModel(config{}, nil, nil, nil)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m1, ok := updated.(model)
	if !ok {
		t.Fatalf("expected model after first key update")
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m2, ok := updated.(model)
	if !ok {
		t.Fatalf("expected model after second key update")
	}

	if got := m2.query; got != "jk" {
		t.Fatalf("query = %q, want %q", got, "jk")
	}
	if got := m2.input.Value(); got != "jk" {
		t.Fatalf("input value = %q, want %q", got, "jk")
	}
}

func TestModelUpdateArrowNavigationStillWorks(t *testing.T) {
	m := newModel(config{}, nil, nil, nil)
	m.filtered = []candidate.FilteredCandidate{{Index: 0}, {Index: 1}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m1, ok := updated.(model)
	if !ok {
		t.Fatalf("expected model after down key update")
	}
	if m1.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", m1.cursor)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyUp})
	m2, ok := updated.(model)
	if !ok {
		t.Fatalf("expected model after up key update")
	}
	if m2.cursor != 0 {
		t.Fatalf("cursor after up = %d, want 0", m2.cursor)
	}
}
