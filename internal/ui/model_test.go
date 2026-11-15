package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/naxmefy/gh-pui/internal/config"
	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/layout"
	"github.com/naxmefy/gh-pui/internal/ui/theme"
)

func TestWindowSizeSetsBreakpoint(t *testing.T) {
	th, _ := theme.New(config.Default(), "none")
	keys, _ := NewKeyMap(nil)
	p := &project.Project{Owner: "x", Number: 1, Views: []project.View{{Number: 1, Name: "Board", Layout: "BOARD_LAYOUT", Filter: "-status:Later"}}}
	m := tea.Model(NewModel("me", p, th, keys))

	for width, want := range map[int]layout.Breakpoint{40: layout.XS, 80: layout.SM, 150: layout.LG} {
		m, _ = m.Update(tea.WindowSizeMsg{Width: width, Height: 30})
		got := m.(Model)
		if got.bp != want {
			t.Errorf("width %d: bp %s, want %s", width, got.bp, want)
		}
		view := got.View().Content
		if hasFilter := strings.Contains(view, "-status:Later"); hasFilter != (want >= layout.MD) {
			t.Errorf("width %d: filter shown = %v", width, hasFilter)
		}
	}
}
