package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/naxmefy/gh-pui/internal/gh"
	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/theme"
)

// ProjectLoader loads a project with its items; picked is true when the user chose it in the picker.
type ProjectLoader func(ref gh.ProjectRef, picked bool) (*project.Project, error)

type loadedMsg struct {
	p   *project.Project
	err error
}

// App starts in the picker (or loads a known project right away) and switches to the table once a project is loaded.
type App struct {
	login         string
	theme         theme.Theme
	keys          KeyMap
	load          ProjectLoader
	picker        Picker
	table         *Model
	loading       *gh.ProjectRef
	picked        bool
	width, height int
}

// NewApp opens auto directly when it is set, else shows the picker.
func NewApp(login string, th theme.Theme, keys KeyMap, picker Picker, load ProjectLoader, auto *gh.ProjectRef) App {
	return App{login: login, theme: th, keys: keys, load: load, picker: picker, loading: auto, width: 80, height: 24}
}

func (a App) loadCmd() tea.Cmd {
	ref, picked, load := *a.loading, a.picked, a.load
	return func() tea.Msg {
		p, err := load(ref, picked)
		return loadedMsg{p, err}
	}
}

func (a App) Init() tea.Cmd {
	if a.loading != nil {
		return a.loadCmd()
	}
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		a.width, a.height = ws.Width, ws.Height
		a.picker, _ = a.picker.Update(ws)
	}
	if a.table != nil {
		m, cmd := a.table.Update(msg)
		t := m.(Model)
		a.table = &t
		return a, cmd
	}
	switch msg := msg.(type) {
	case PickedMsg:
		a.loading, a.picked = &msg.Ref, true
		return a, a.loadCmd()
	case loadedMsg:
		ref := a.loading
		a.loading = nil
		if msg.err != nil {
			a.picker.err = fmt.Errorf("%s/%d: %w", ref.Owner, ref.Number, msg.err)
			return a, nil
		}
		m, _ := NewModel(a.login, msg.p, a.theme, a.keys).Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
		t := m.(Model)
		a.table = &t
		return a, nil
	case tea.KeyPressMsg:
		if a.loading != nil {
			if msg.String() == "ctrl+c" {
				return a, tea.Quit
			}
			return a, nil
		}
	}
	var cmd tea.Cmd
	a.picker, cmd = a.picker.Update(msg)
	return a, cmd
}

func (a App) View() tea.View {
	if a.table != nil {
		return a.table.View()
	}
	if a.loading != nil {
		v := tea.NewView(a.theme.Fg("muted").Render(fmt.Sprintf(" Loading %s/%d …", a.loading.Owner, a.loading.Number)))
		v.AltScreen = true
		return v
	}
	return a.picker.View()
}
