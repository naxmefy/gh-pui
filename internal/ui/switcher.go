package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/naxmefy/gh-pui/internal/gh"
	"github.com/naxmefy/gh-pui/internal/ui/theme"
)

// ProjectSource fetches what the switcher lists: the viewer's and their orgs' projects, and the current repo's.
type ProjectSource func() (viewer, repo []gh.ProjectRef, err error)

type switcherDataMsg struct {
	viewer, repo []gh.ProjectRef
	err          error
}

var scopes = []string{"All", "This repo", "Org", "Me"}

// Switcher is the ctrl+p overlay. It fetches once per session and keeps the list for later openings
// (shortcut: no refresh until restart; pinned and recent projects come with v0.6).
type Switcher struct {
	theme          theme.Theme
	login          string
	viewer, repo   []gh.ProjectRef
	loaded         bool
	scope          int
	filter         string
	cursor, offset int
	err            error
}

func (s Switcher) refs() []gh.ProjectRef {
	var out []gh.ProjectRef
	switch scopes[s.scope] {
	case "This repo":
		return s.repo
	case "All":
		seen := map[string]bool{}
		for _, r := range append(append([]gh.ProjectRef{}, s.viewer...), s.repo...) {
			if k := fmt.Sprintf("%s/%d", r.Owner, r.Number); !seen[k] {
				seen[k] = true
				out = append(out, r)
			}
		}
	default: // Org | Me
		for _, r := range s.viewer {
			if (r.Owner == s.login) == (scopes[s.scope] == "Me") {
				out = append(out, r)
			}
		}
	}
	return out
}

func (s Switcher) visible() []Entry { return match(ProjectEntries(s.refs()), s.filter) }

// open resets the input for a new opening and keeps the fetched list.
func (s Switcher) open() Switcher {
	s.filter, s.cursor, s.offset, s.err = "", 0, 0, nil
	return s
}

const switcherList = 6 // cards shown at once

// Update returns closed=true when the overlay should go away (esc on an empty filter).
func (s Switcher) Update(msg tea.Msg) (Switcher, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case switcherDataMsg:
		s.viewer, s.repo, s.loaded, s.err = msg.viewer, msg.repo, true, msg.err
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return s, tea.Quit, false
		case "esc":
			if s.filter == "" {
				return s, nil, true
			}
			s.filter, s.cursor = "", 0
		case "tab":
			s.scope, s.cursor = (s.scope+1)%len(scopes), 0
		case "shift+tab":
			s.scope, s.cursor = (s.scope+len(scopes)-1)%len(scopes), 0
		case "up", "ctrl+k":
			s.cursor--
		case "down", "ctrl+j":
			s.cursor++
		case "backspace":
			if r := []rune(s.filter); len(r) > 0 {
				s.filter, s.cursor = string(r[:len(r)-1]), 0
			}
		case "enter":
			if vis := s.visible(); s.cursor < len(vis) {
				ref := *vis[s.cursor].Ref
				return s, func() tea.Msg { return PickedMsg{ref} }, true
			}
		default:
			if t := msg.Text; t != "" {
				s.filter, s.cursor = s.filter+t, 0
			}
		}
	}
	s.cursor = max(0, min(s.cursor, len(s.visible())-1))
	s.offset = max(min(s.offset, s.cursor), s.cursor-switcherList+1, 0)
	return s, nil, false
}

// Overlay draws the switcher box centered over base, a full screen of width × height.
func (s Switcher) Overlay(base string, width, height int) string {
	t := s.theme
	w := max(20, min(width-4, 64))
	var tabs []string
	for i, sc := range scopes {
		if i == s.scope {
			tabs = append(tabs, t.Fg("accent").Underline(true).Render(sc))
		} else {
			tabs = append(tabs, t.Fg("muted").Render(sc))
		}
	}
	lines := []string{t.Fg("accent").Bold(true).Render(" Switch project"), " " + strings.Join(tabs, "  "), " > " + s.filter + "▌", ""}
	if s.loaded {
		lines = append(lines, renderEntries(t, s.visible(), s.cursor, s.offset, min(switcherList, max(1, (height-12)/2)))...)
	} else {
		lines = append(lines, t.Fg("muted").Render("   Loading projects…"))
	}
	status := t.Fg("muted").Render(" tab scope · enter open · esc close")
	if s.err != nil {
		status = t.Fg("danger").Render(" " + s.err.Error())
	}
	lines = append(lines, "", status)
	return overlay(t, base, lines, w, width, height)
}

// overlay draws lines in a rounded box of width w, centered horizontally and in the upper third of base.
func overlay(t theme.Theme, base string, lines []string, w, width, height int) string {
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, w-2, "…")
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Color("border")).Width(w).
		Render(strings.Join(lines, "\n"))
	x, y := max(0, (width-lipgloss.Width(box))/2), max(0, (height-lipgloss.Height(box))/3)
	return lipgloss.NewCompositor(lipgloss.NewLayer(base), lipgloss.NewLayer(box).X(x).Y(y).Z(1)).Render()
}
