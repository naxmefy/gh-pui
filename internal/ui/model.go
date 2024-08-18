package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
}

func NewModel() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) View() string {
	return fmt.Sprintf("\n\n     Hi. This the program - press Q, ESC or CTRL+C to quit")
}
