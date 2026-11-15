package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/theme"
)

type Model struct {
	login   string
	project *project.Project
	theme   theme.Theme
	keys    KeyMap
}

func NewModel(login string, p *project.Project, th theme.Theme, keys KeyMap) Model {
	return Model{login: login, project: p, theme: th, keys: keys}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := message.(tea.KeyPressMsg); ok && key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	return m, nil
}

// View is a placeholder summary until the table view lands.
func (m Model) View() tea.View {
	p, t := m.project, m.theme
	muted := t.Fg("muted")
	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s  %s\n\n", t.Fg("accent").Bold(true).Render(fmt.Sprintf("%s/%d  %s", p.Owner, p.Number, p.Title)), muted.Render("as "+m.login))
	fmt.Fprintf(&b, "  %s\n\n", muted.Render(fmt.Sprintf("%d items · %d fields · %d views", len(p.Items), len(p.Fields), len(p.Views))))
	for _, v := range p.Views {
		fmt.Fprintf(&b, "  %d  %-24s %-15s %s\n", v.Number, v.Name, v.Layout, muted.Render(v.Filter))
	}
	fmt.Fprintf(&b, "\n  %s\n", muted.Render(m.keys.Quit.Help().Key+" to quit"))
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}
