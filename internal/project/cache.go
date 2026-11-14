package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CacheDir is $XDG_CACHE_HOME/gh-pui, falling back to ~/.cache/gh-pui like gh does on every platform.
func CacheDir() (string, error) {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return filepath.Join(d, "gh-pui"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "gh-pui"), nil
}

func cachePath(owner string, number int) (string, error) {
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "projects", owner, fmt.Sprintf("%d.json", number)), nil
}

// Save writes the project to the cache via temp file + rename, so a crash never leaves a half-written cache.
func Save(p *Project) error {
	path, err := cachePath(p.Owner, p.Number)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Load returns the cached project, or an error wrapping os.ErrNotExist when there is none.
func Load(owner string, number int) (*Project, error) {
	path, err := cachePath(owner, number)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("corrupt cache %s: %w", path, err)
	}
	return &p, nil
}
