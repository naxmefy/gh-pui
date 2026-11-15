package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Choice is the project picked for a repo, kept in projects.yml so the picker is skipped next time.
type Choice struct {
	Owner  string `yaml:"owner"`
	Number int    `yaml:"number"`
}

func choicesPath() (string, error) {
	dir, err := Dir()
	return filepath.Join(dir, "projects.yml"), err
}

// LoadChoices reads projects.yml (repo "owner/name" -> project); a missing file is an empty map.
func LoadChoices() (map[string]Choice, error) {
	path, err := choicesPath()
	if err != nil {
		return nil, err
	}
	choices := map[string]Choice{}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return choices, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &choices); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return choices, nil
}

// SaveChoice remembers the project for a repo; the file is rewritten whole, so comments in it are lost.
func SaveChoice(repo string, c Choice) error {
	choices, err := LoadChoices()
	if err != nil {
		return err
	}
	if choices[repo] == c {
		return nil
	}
	choices[repo] = c
	data, err := yaml.Marshal(choices)
	if err != nil {
		return err
	}
	path, err := choicesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
