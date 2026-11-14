package ui

import (
	tea "charm.land/bubbletea/v2"
)

type Model struct {
	login string
}

func NewModel(login string) Model {
	return Model{login: login}
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

func (m Model) View() tea.View {
	v := tea.NewView("\n\n     Hi " + m.login + ". This is gh pui - press q, esc or ctrl+c to quit")
	v.AltScreen = true
	return v
}
