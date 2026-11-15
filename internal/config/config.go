// Package config loads ~/.config/gh-pui/config.yml (or $XDG_CONFIG_HOME/gh-pui/config.yml), like gh does for its own config.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Theme   string              `yaml:"theme"`   // auto | ansi | none
	Symbols string              `yaml:"symbols"` // nerdfont | unicode | ascii
	Layout  string              `yaml:"layout"`  // table | board | roadmap
	Colors  map[string]string   `yaml:"colors"`  // role -> ANSI index or "#rrggbb"
	Keys    map[string][]string `yaml:"keys"`    // action -> key strokes
}

func Default() Config {
	return Config{Theme: "auto", Symbols: "unicode", Layout: "table"}
}

// Dir is $XDG_CONFIG_HOME/gh-pui, falling back to ~/.config/gh-pui on every platform.
func Dir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "gh-pui"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "gh-pui"), nil
}

// Load reads config.yml over the defaults; a missing file is not an error.
func Load() (Config, error) {
	dir, err := Dir()
	if err != nil {
		return Config{}, err
	}
	path := filepath.Join(dir, "config.yml")
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

func (c Config) validate() error {
	for _, f := range []struct {
		name, value string
		allowed     []string
	}{
		{"theme", c.Theme, []string{"auto", "ansi", "none"}},
		{"symbols", c.Symbols, []string{"nerdfont", "unicode", "ascii"}},
		{"layout", c.Layout, []string{"table", "board", "roadmap"}},
	} {
		if !contains(f.allowed, f.value) {
			return fmt.Errorf("%s: %q is not one of %v", f.name, f.value, f.allowed)
		}
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
