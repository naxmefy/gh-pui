package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/naxmefy/gh-pui/internal/project"
)

type Model struct {
	login   string
	project *project.Project
}

func NewModel(login string, p *project.Project) Model {
	return Model{login: login, project: p}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

// View is a placeholder summary until the table view lands.
func (m Model) View() tea.View {
	p := m.project
	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s/%d  %s  (as %s)\n\n", p.Owner, p.Number, p.Title, m.login)
	fmt.Fprintf(&b, "  %d items · %d fields · %d views\n\n", len(p.Items), len(p.Fields), len(p.Views))
	for _, v := range p.Views {
		fmt.Fprintf(&b, "  %d  %-24s %-15s %s\n", v.Number, v.Name, v.Layout, v.Filter)
	}
	b.WriteString("\n  q to quit\n")
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}
