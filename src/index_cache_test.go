package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func withIndexCachePath(t *testing.T, path string) {
	t.Helper()
	old := indexCachePathOverride
	indexCachePathOverride = path
	t.Cleanup(func() {
		indexCachePathOverride = old
	})
}

func TestIndexCacheRoundTrip(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "last_index.gob")
	withIndexCachePath(t, cachePath)

	cfg := ProducerConfig{
		Root:         "/repo/project",
		Pattern:      defaultRGPattern,
		NoIgnore:     false,
		ExcludeTests: true,
		Excludes:     []string{"vendor/**"},
	}
	candidates := []Candidate{
		{ID: 1, File: "a.go", Line: 10, Col: 2, Text: "func A() {}", Key: "A", LangID: LangGo},
		{ID: 2, File: "b.ts", Line: 5, Col: 1, Text: "export const b = 1", Key: "b", LangID: LangTypeScript},
	}

	if err := SaveIndexCache(cfg, candidates); err != nil {
		t.Fatalf("SaveIndexCache failed: %v", err)
	}

	got, ok, err := LoadIndexCache(cfg)
	if err != nil {
		t.Fatalf("LoadIndexCache failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected matching cache to load")
	}
	if !reflect.DeepEqual(got, candidates) {
		t.Fatalf("loaded candidates do not match saved candidates")
	}
}

func TestIndexCacheOnlyKeepsLastIndex(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "last_index.gob")
	withIndexCachePath(t, cachePath)

	cfgA := ProducerConfig{Root: "/repo/a", Pattern: defaultRGPattern}
	cfgB := ProducerConfig{Root: "/repo/b", Pattern: defaultRGPattern}

	if err := SaveIndexCache(cfgA, []Candidate{{ID: 1, File: "a.go", Key: "A"}}); err != nil {
		t.Fatalf("SaveIndexCache A failed: %v", err)
	}
	if err := SaveIndexCache(cfgB, []Candidate{{ID: 1, File: "b.go", Key: "B"}}); err != nil {
		t.Fatalf("SaveIndexCache B failed: %v", err)
	}

	if _, ok, err := LoadIndexCache(cfgA); err != nil {
		t.Fatalf("LoadIndexCache A failed: %v", err)
	} else if ok {
		t.Fatalf("expected cache miss for older root")
	}

	gotB, ok, err := LoadIndexCache(cfgB)
	if err != nil {
		t.Fatalf("LoadIndexCache B failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected cache hit for latest root")
	}
	if len(gotB) != 1 || gotB[0].File != "b.go" {
		t.Fatalf("unexpected cache payload for latest root")
	}
}
