package main

import (
	"bufio"
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"

	"snav/internal/candidate"
)

const indexCacheVersion = 1

var indexCachePathOverride string

type diskIndexCache struct {
	Version      int
	Root         string
	Pattern      string
	NoIgnore     bool
	ExcludeTests bool
	Excludes     []string
	Candidates   []candidate.Candidate
}

func LoadIndexCache(cfg candidate.ProducerConfig) ([]candidate.Candidate, bool, error) {
	path, err := indexCachePath()
	if err != nil {
		return nil, false, err
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer f.Close()

	var disk diskIndexCache
	if err := gob.NewDecoder(bufio.NewReaderSize(f, 1<<20)).Decode(&disk); err != nil {
		return nil, false, err
	}

	if !indexCacheMatches(disk, cfg) {
		return nil, false, nil
	}

	return disk.Candidates, true, nil
}

func SaveIndexCache(cfg candidate.ProducerConfig, candidates []candidate.Candidate) error {
	path, err := indexCachePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	writer := bufio.NewWriterSize(f, 1<<20)
	disk := diskIndexCache{
		Version:      indexCacheVersion,
		Root:         filepath.Clean(cfg.Root),
		Pattern:      cfg.Pattern,
		NoIgnore:     cfg.NoIgnore,
		ExcludeTests: cfg.ExcludeTests,
		Excludes:     append([]string(nil), cfg.Excludes...),
		Candidates:   candidates,
	}
	if err := gob.NewEncoder(writer).Encode(&disk); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := writer.Flush(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return nil
}

func indexCacheMatches(disk diskIndexCache, cfg candidate.ProducerConfig) bool {
	if disk.Version != indexCacheVersion {
		return false
	}
	if filepath.Clean(disk.Root) != filepath.Clean(cfg.Root) {
		return false
	}
	if disk.Pattern != cfg.Pattern || disk.NoIgnore != cfg.NoIgnore || disk.ExcludeTests != cfg.ExcludeTests {
		return false
	}
	if len(disk.Excludes) != len(cfg.Excludes) {
		return false
	}
	for i := range disk.Excludes {
		if disk.Excludes[i] != cfg.Excludes[i] {
			return false
		}
	}
	return true
}

func indexCachePath() (string, error) {
	if indexCachePathOverride != "" {
		return indexCachePathOverride, nil
	}

	root, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "snav", "last_index.gob"), nil
}
