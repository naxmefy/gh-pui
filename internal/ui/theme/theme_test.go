package theme

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/naxmefy/gh-pui/internal/config"
)

func TestModes(t *testing.T) {
	cfg := config.Default()

	t.Setenv("NO_COLOR", "")
	th, err := New(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if th.Color("accent") != lipgloss.Color("12") {
		t.Errorf("auto without NO_COLOR should use ANSI, got %v", th.Color("accent"))
	}

	t.Setenv("NO_COLOR", "1")
	if th, _ = New(cfg, ""); th.Color("accent") != (lipgloss.NoColor{}) {
		t.Errorf("auto with NO_COLOR should drop colors, got %v", th.Color("accent"))
	}
	if th, _ = New(cfg, "ansi"); th.Color("accent") != lipgloss.Color("12") {
		t.Errorf("--theme ansi should color despite NO_COLOR, got %v", th.Color("accent"))
	}
	if th, _ = New(cfg, "none"); th.Color("danger") != (lipgloss.NoColor{}) {
		t.Errorf("none should drop colors")
	}
	if _, err := New(cfg, "rainbow"); err == nil {
		t.Error("unknown --theme should fail")
	}
}

func TestCustomColorsAndSymbols(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	cfg := config.Default()
	cfg.Colors = map[string]string{"accent": "#ff8800"}
	cfg.Symbols = "ascii"
	th, err := New(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if th.Color("accent") != lipgloss.Color("#ff8800") || th.Color("muted") != lipgloss.Color("8") {
		t.Errorf("custom color not applied")
	}
	if th.Symbol("issue") != "o" {
		t.Errorf("ascii symbols not applied, got %q", th.Symbol("issue"))
	}

	cfg.Colors = map[string]string{"acent": "1"}
	if _, err := New(cfg, ""); err == nil {
		t.Error("unknown color role should fail")
	}
}
