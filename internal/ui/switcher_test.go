package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/naxmefy/gh-pui/internal/config"
	"github.com/naxmefy/gh-pui/internal/gh"
	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/theme"
)

func TestSwitcherScopesAndSwitch(t *testing.T) {
	th, _ := theme.New(config.Default(), "none")
	keys, _ := NewKeyMap(nil)
	var loaded []gh.ProjectRef
	var picks []bool
	load := func(ref gh.ProjectRef, picked bool) (*project.Project, error) {
		loaded, picks = append(loaded, ref), append(picks, picked)
		p := testProject()
		p.Owner, p.Number, p.Title = ref.Owner, ref.Number, "loaded"
		return p, nil
	}
	fetches := 0
	source := func() ([]gh.ProjectRef, []gh.ProjectRef, error) {
		fetches++
		return []gh.ProjectRef{{Owner: "me", Number: 1, Title: "Mine"}, {Owner: "acme", Number: 2, Title: "OrgProj"}},
			[]gh.ProjectRef{{Owner: "acme", Number: 2, Title: "OrgProj"}, {Owner: "other", Number: 5, Title: "RepoProj"}}, nil
	}
	var m tea.Model = NewApp("me", th, keys, NewPicker(th, nil, Page{}), load, source, nil, &gh.ProjectRef{Owner: "me", Number: 1})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m, _ = m.Update(m.Init()())

	m = drive(m, "ctrl+p")
	s := view(m)
	if !strings.Contains(s, "Switch project") || strings.Count(s, "OrgProj") != 1 || !strings.Contains(s, "Mine") || !strings.Contains(s, "RepoProj") {
		t.Fatalf("All scope wrong (acme/2 must appear once):\n%s", s)
	}
	for _, c := range []struct {
		want, not []string
	}{
		{[]string{"OrgProj", "RepoProj"}, []string{"Mine"}}, // This repo
		{[]string{"OrgProj"}, []string{"Mine", "RepoProj"}}, // Org
		{[]string{"Mine"}, []string{"OrgProj", "RepoProj"}}, // Me
	} {
		m = drive(m, "tab")
		s = view(m)
		for _, w := range c.want {
			if !strings.Contains(s, w) {
				t.Errorf("scope lacks %s:\n%s", w, s)
			}
		}
		for _, n := range c.not {
			if strings.Contains(s, n) {
				t.Errorf("scope shows %s:\n%s", n, s)
			}
		}
	}

	m = drive(m, "esc")
	if strings.Contains(view(m), "Switch project") {
		t.Fatal("esc should close the switcher")
	}
	m = drive(m, "ctrl+p", "r", "e", "p", "o", "enter") // reopens in the last scope (Me): nothing matches
	if len(loaded) != 1 {
		t.Fatalf("enter on no match must not load, loaded %v", loaded)
	}
	m = drive(m, "tab", "enter") // back to All, filter "repo" kept → RepoProj
	if len(loaded) != 2 || loaded[1].Number != 5 || !picks[1] {
		t.Fatalf("loaded %v picks %v", loaded, picks)
	}
	if s := view(m); !strings.Contains(s, "#5 loaded") || strings.Contains(s, "Switch project") {
		t.Errorf("table of the new project not shown:\n%s", s)
	}
	if fetches != 1 {
		t.Errorf("source fetched %d times, want once per session", fetches)
	}
}
