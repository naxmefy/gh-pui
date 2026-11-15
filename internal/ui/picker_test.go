package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/naxmefy/gh-pui/internal/config"
	"github.com/naxmefy/gh-pui/internal/gh"
	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/theme"
)

func keyMsg(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "ctrl+p":
		return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: rune(k[0]), Text: k}
}

// drive feeds keys to a model and runs every returned command synchronously, like the runtime would.
func drive(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		var cmd tea.Cmd
		m, cmd = m.Update(keyMsg(k))
		for cmd != nil {
			msg := cmd()
			if _, quit := msg.(tea.QuitMsg); quit {
				break
			}
			m, cmd = m.Update(msg)
		}
	}
	return m
}

func view(m tea.Model) string { return ansi.Strip(m.View().Content) }

func newTestApp(t *testing.T, loadErr error) (tea.Model, *[]gh.ProjectRef) {
	t.Helper()
	th, _ := theme.New(config.Default(), "none")
	keys, _ := NewKeyMap(nil)
	lister := func(owner string) ([]gh.ProjectRef, error) {
		return []gh.ProjectRef{{Owner: owner, Number: 1, Title: "Roadmap"}, {Owner: owner, Number: 2, Title: "Old", Closed: true}, {Owner: owner, Number: 3, Title: "Bugs"}}, nil
	}
	var loaded []gh.ProjectRef
	load := func(ref gh.ProjectRef, picked bool) (*project.Project, error) {
		loaded = append(loaded, ref)
		if loadErr != nil {
			return nil, loadErr
		}
		p := testProject()
		p.Owner, p.Number, p.Title = ref.Owner, ref.Number, "loaded"
		return p, nil
	}
	first := Page{Title: "Choose an owner", Entries: OwnerEntries("me", "acme", "me")}
	app := NewApp("me", th, keys, NewPicker(th, lister, first), load, nil, nil, nil)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m, &loaded
}

func TestMatchFiltersAndKeepsOrderWhenEmpty(t *testing.T) {
	refs := []gh.ProjectRef{{Owner: "a", Number: 1, Title: "Roadmap"}, {Owner: "a", Number: 2, Title: "gh-pui"}, {Owner: "a", Number: 3, Title: "pull ui"}}
	es := ProjectEntries(refs)
	if got := match(es, ""); len(got) != 3 || got[0].Ref.Title != "Roadmap" {
		t.Errorf("empty filter: %v", got)
	}
	if got := match(es, "pui"); len(got) != 2 {
		t.Errorf("pui: want 2 matches, got %d", len(got))
	}
	if got := match(es, "rdmp"); len(got) != 1 || got[0].Ref.Title != "Roadmap" {
		t.Errorf("rdmp: %v", got)
	}
}

func TestPickerOwnerToProjectToTable(t *testing.T) {
	m, loaded := newTestApp(t, nil)
	s := view(m)
	if !strings.Contains(s, "me") || !strings.Contains(s, "acme") || strings.Count(s, "▌ ") != 1 {
		t.Fatalf("owner page wrong (duplicates must collapse):\n%s", s)
	}

	m = drive(m, "down", "enter") // acme → its projects, closed ones hidden
	s = view(m)
	if !strings.Contains(s, "Projects of acme") || !strings.Contains(s, "Bugs") || strings.Contains(s, "Old") {
		t.Fatalf("project page wrong:\n%s", s)
	}

	m = drive(m, "b", "u", "g", "enter") // filter to Bugs and open it
	if len(*loaded) != 1 || (*loaded)[0] != (gh.ProjectRef{Owner: "acme", Number: 3, Title: "Bugs"}) {
		t.Fatalf("loaded %v", *loaded)
	}
	if s := view(m); !strings.Contains(s, "#3 loaded") {
		t.Errorf("table not shown:\n%s", s)
	}
}

func TestPickerEscGoesBackAndClearsFilter(t *testing.T) {
	m, _ := newTestApp(t, nil)
	m = drive(m, "enter", "z")
	if s := view(m); !strings.Contains(s, "nothing matches") {
		t.Fatalf("filter z should match nothing:\n%s", s)
	}
	m = drive(m, "esc")
	if s := view(m); !strings.Contains(s, "Roadmap") {
		t.Errorf("esc should clear the filter:\n%s", s)
	}
	m = drive(m, "esc")
	if s := view(m); !strings.Contains(s, "Choose an owner") {
		t.Errorf("esc should go back to owners:\n%s", s)
	}
}

func TestLoadErrorStaysInPicker(t *testing.T) {
	m, _ := newTestApp(t, errors.New("boom"))
	s := view(drive(m, "enter", "enter"))
	if !strings.Contains(s, "me/1: boom") || !strings.Contains(s, "Roadmap") {
		t.Errorf("error not shown in picker:\n%s", s)
	}
}

func TestAutoLoadSkipsPicker(t *testing.T) {
	th, _ := theme.New(config.Default(), "none")
	keys, _ := NewKeyMap(nil)
	load := func(ref gh.ProjectRef, picked bool) (*project.Project, error) {
		if picked {
			t.Error("auto load must not count as picked")
		}
		p := testProject()
		p.Owner, p.Number, p.Title = ref.Owner, ref.Number, "auto"
		return p, nil
	}
	var m tea.Model = NewApp("me", th, keys, NewPicker(th, nil, Page{}), load, nil, nil, &gh.ProjectRef{Owner: "x", Number: 9})
	if s := view(m); !strings.Contains(s, "Loading x/9") {
		t.Fatalf("loading screen missing:\n%s", s)
	}
	m, _ = m.Update(m.Init()())
	if s := view(m); !strings.Contains(s, "#9 auto") {
		t.Errorf("table not shown:\n%s", s)
	}
}
