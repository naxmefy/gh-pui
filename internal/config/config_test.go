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

func TestChoicesRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if c, err := LoadChoices(); err != nil || len(c) != 0 {
		t.Fatalf("missing file: %v %v", c, err)
	}
	if err := SaveChoice("a/b", Choice{"a", 3}); err != nil {
		t.Fatal(err)
	}
	if err := SaveChoice("c/d", Choice{"org", 7}); err != nil {
		t.Fatal(err)
	}
	c, err := LoadChoices()
	if err != nil || c["a/b"] != (Choice{"a", 3}) || c["c/d"] != (Choice{"org", 7}) {
		t.Errorf("got %v %v", c, err)
	}
}
