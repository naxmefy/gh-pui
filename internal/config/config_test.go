package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "gh-pui"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gh-pui", "config.yml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("got %+v", cfg)
	}
}

func TestLoadOverridesDefaults(t *testing.T) {
	writeConfig(t, "symbols: ascii\ncolors:\n  accent: \"#ff8800\"\nkeys:\n  quit: [\"x\"]\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "auto" || cfg.Symbols != "ascii" || cfg.Colors["accent"] != "#ff8800" || cfg.Keys["quit"][0] != "x" {
		t.Errorf("got %+v", cfg)
	}
}

func TestLoadRejectsUnknownValues(t *testing.T) {
	writeConfig(t, "theme: rainbow\n")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "theme") || !strings.Contains(err.Error(), "config.yml") {
		t.Errorf("got %v", err)
	}
}

func TestProjectsRoundTripAndSettings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if p, err := LoadProjects(); err != nil || len(p.Repos) != 0 {
		t.Fatalf("missing file: %v %v", p, err)
	}
	writeFile(t, "projects.yml", "projects:\n  acme/7:\n    sum: []\n    views:\n      Backlog: {hierarchy: false, slice: Priority}\n")
	if err := SaveChoice("a/b", Choice{"a", 3}); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProjects()
	if err != nil || p.Repos["a/b"] != (Choice{"a", 3}) {
		t.Fatalf("repos: %v %v", p.Repos, err)
	}
	s := p.Settings("acme", 7) // settings survive SaveChoice
	if s.Sum == nil || len(*s.Sum) != 0 || *s.Views["Backlog"].Hierarchy || s.Views["Backlog"].Slice != "Priority" {
		t.Errorf("settings: %+v", s)
	}
	if p.Settings("acme", 8).Sum != nil {
		t.Error("unknown project should have no settings")
	}
}

func TestProjectsReadsFirstFormat(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeFile(t, "projects.yml", "naxmefy/gh-pui:\n  owner: naxmefy\n  number: 18\n")
	p, err := LoadProjects()
	if err != nil || p.Repos["naxmefy/gh-pui"] != (Choice{"naxmefy", 18}) {
		t.Errorf("got %v %v", p, err)
	}
}

func writeFile(t *testing.T, name, content string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "gh-pui")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
