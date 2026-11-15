// Package theme maps color roles and symbols to what the terminal offers.
// Colors default to the terminal's own ANSI-16 palette, so the user's terminal scheme decides the look.
package theme

import (
	"fmt"
	"image/color"
	"os"
	"sort"

	"charm.land/lipgloss/v2"
	"github.com/naxmefy/gh-pui/internal/config"
)

// ansi maps every color role to an ANSI-16 index; code only ever asks for roles, never for concrete colors.
var ansi = map[string]string{
	"accent":          "12",
	"muted":           "8",
	"border":          "8",
	"success":         "2",
	"warning":         "3",
	"danger":          "1",
	"status.todo":     "7",
	"status.progress": "3",
	"status.review":   "5",
	"status.done":     "2",
	"status.blocked":  "1",
}

var symbolSets = map[string]map[string]string{
	"unicode": {
		"issue": "○", "issue.closed": "●", "pr": "◇", "pr.merged": "◆", "draft": "◌",
		"expanded": "▾", "collapsed": "▸",
	},
	"ascii": {
		"issue": "o", "issue.closed": "x", "pr": "p", "pr.merged": "m", "draft": "d",
		"expanded": "v", "collapsed": ">",
	},
}

type Theme struct {
	colors  map[string]color.Color
	symbols map[string]string
}

// New builds the theme from the config; override is the --theme flag ("" = use config).
// Mode auto honors NO_COLOR, mode ansi always colors, mode none never does.
func New(cfg config.Config, override string) (Theme, error) {
	mode := cfg.Theme
	if override != "" {
		mode = override
	}
	switch mode {
	case "auto", "ansi", "none":
	default:
		return Theme{}, fmt.Errorf("theme %q is not one of auto, ansi, none", mode)
	}
	if mode == "auto" && os.Getenv("NO_COLOR") != "" {
		mode = "none"
	}

	for role := range cfg.Colors {
		if _, ok := ansi[role]; !ok {
			return Theme{}, fmt.Errorf("colors: unknown role %q, known: %v", role, Roles())
		}
	}

	t := Theme{colors: map[string]color.Color{}, symbols: symbolSets["unicode"]}
	for role, value := range ansi {
		if v, ok := cfg.Colors[role]; ok {
			value = v
		}
		if mode == "none" {
			t.colors[role] = lipgloss.NoColor{}
		} else {
			t.colors[role] = lipgloss.Color(value)
		}
	}
	// shortcut: nerdfont reuses the unicode set; real Nerd Font glyphs are planned for v1.0.
	if s, ok := symbolSets[cfg.Symbols]; ok {
		t.symbols = s
	}
	return t, nil
}

// Roles lists the color roles that config.yml may override.
func Roles() []string {
	roles := make([]string, 0, len(ansi))
	for r := range ansi {
		roles = append(roles, r)
	}
	sort.Strings(roles)
	return roles
}

// Fg is a style with the role's foreground color.
func (t Theme) Fg(role string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Color(role))
}

func (t Theme) Color(role string) color.Color {
	if c, ok := t.colors[role]; ok {
		return c
	}
	return lipgloss.NoColor{}
}

func (t Theme) Symbol(name string) string {
	return t.symbols[name]
}
