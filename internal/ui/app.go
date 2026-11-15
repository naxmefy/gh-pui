package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/naxmefy/gh-pui/internal/gh"
	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/theme"
)

// ProjectLoader loads a project with its items; picked is true when the user chose it in the picker or switcher.
type ProjectLoader func(ref gh.ProjectRef, picked bool) (*project.Project, error)

type loadedMsg struct {
	p   *project.Project
	err error
}

// App starts in the picker (or loads a known project right away), switches to the table once a project
// is loaded, and opens the switcher overlay over the table.
type App struct {
	login         string
	theme         theme.Theme
	keys          KeyMap
	load          ProjectLoader
	source        ProjectSource
	save          SaveFunc
	picker        Picker
	switcher      Switcher
	switching     bool
	fetched       bool // switcher list requested once
	table         *Model
	loading       *gh.ProjectRef
	picked        bool
	width, height int
}

// NewApp opens auto directly when it is set, else shows the picker.
func NewApp(login string, th theme.Theme, keys KeyMap, picker Picker, load ProjectLoader, source ProjectSource, save SaveFunc, auto *gh.ProjectRef) App {
	return App{login: login, theme: th, keys: keys, load: load, source: source, save: save, picker: picker, loading: auto,
		switcher: Switcher{theme: th, login: login}, width: 80, height: 24}
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

// openSwitcher shows the overlay and fetches its list on first use.
func (a App) openSwitcher() (App, tea.Cmd) {
	a.switching, a.switcher = true, a.switcher.open()
	if a.fetched {
		return a, nil
	}
	a.fetched = true
	source := a.source
	return a, func() tea.Msg {
		viewer, repo, err := source()
		return switcherDataMsg{viewer, repo, err}
	}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.picker, _ = a.picker.Update(msg)
	case PickedMsg:
		a.loading, a.picked, a.switching = &msg.Ref, true, false
		return a, a.loadCmd()
	case loadedMsg:
		ref := a.loading
		a.loading = nil
		if msg.err != nil {
			err := fmt.Errorf("%s/%d: %w", ref.Owner, ref.Number, msg.err)
			if a.table == nil {
				a.picker.err = err
				return a, nil
			}
			a, cmd := a.openSwitcher()
			a.switcher.err = err
			return a, cmd
		}
		m, _ := NewModel(a.login, msg.p, a.theme, a.keys).WithSave(a.save).Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
		t := m.(Model)
		a.table = &t
		return a, nil
	case switcherDataMsg:
		a.switcher, _, _ = a.switcher.Update(msg)
		return a, nil
	case tea.KeyPressMsg:
		switch {
		case a.loading != nil:
			if msg.String() == "ctrl+c" {
				return a, tea.Quit
			}
			return a, nil
		case a.switching && key.Matches(msg, a.keys.Switch):
			a.switching = false
			return a, nil
		case a.switching:
			var cmd tea.Cmd
			var closed bool
			a.switcher, cmd, closed = a.switcher.Update(msg)
			a.switching = !closed
			return a, cmd
		case a.table != nil && !a.table.editing() && key.Matches(msg, a.keys.Switch):
			return a.openSwitcher()
		}
	}
	if a.table != nil {
		m, cmd := a.table.Update(msg)
		t := m.(Model)
		a.table = &t
		return a, cmd
	}
	var cmd tea.Cmd
	a.picker, cmd = a.picker.Update(msg)
	return a, cmd
}

func (a App) View() tea.View {
	if a.loading != nil {
		v := tea.NewView(a.theme.Fg("muted").Render(fmt.Sprintf(" Loading %s/%d …", a.loading.Owner, a.loading.Number)))
		v.AltScreen = true
		return v
	}
	if a.table == nil {
		return a.picker.View()
	}
	v := a.table.View()
	if a.switching {
		v.SetContent(a.switcher.Overlay(v.Content, a.width, a.height))
	}
	return v
}
