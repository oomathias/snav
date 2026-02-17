package main

import "testing"

func TestLoadThemePalette_Known(t *testing.T) {
	palette, err := LoadThemePalette("dracula")
	if err != nil {
		t.Fatalf("expected dracula theme to load: %v", err)
	}
	if palette.Keyword == "" || palette.Text == "" || palette.SelectionBG == "" {
		t.Fatalf("theme palette has empty core colors: %+v", palette)
	}
}

func TestLoadThemePalette_Unknown(t *testing.T) {
	if _, err := LoadThemePalette("this-theme-does-not-exist"); err == nil {
		t.Fatalf("expected unknown theme error")
	}
}
