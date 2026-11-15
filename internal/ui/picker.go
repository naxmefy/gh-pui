package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
	"github.com/naxmefy/gh-pui/internal/gh"
	"github.com/naxmefy/gh-pui/internal/ui/theme"
)

// Entry is one line in the picker: a project, or an owner whose projects open on enter.
type Entry struct {
	Owner string
	Ref   *gh.ProjectRef
}

func ProjectEntries(refs []gh.ProjectRef) []Entry {
	var out []Entry
	for i := range refs {
		if !refs[i].Closed {
			out = append(out, Entry{Ref: &refs[i]})
		}
	}
	return out
}

func OwnerEntries(owners ...string) []Entry {
	var out []Entry
	seen := map[string]bool{}
	for _, o := range owners {
		if o != "" && !seen[o] {
			seen[o] = true
			out = append(out, Entry{Owner: o})
		}
	}
	return out
}

func (e Entry) text() string {
	if e.Ref != nil {
		return fmt.Sprintf("%s %s/%d", e.Ref.Title, e.Ref.Owner, e.Ref.Number)
	}
	return e.Owner
}

// Page is one list in the picker; enter on an owner pushes that owner's projects as a new page.
type Page struct {
	Title, Subtitle string
	Entries         []Entry
	filter          string
	cursor, offset  int
}

func (p Page) visible() []Entry { return match(p.Entries, p.filter) }

type entrySource []Entry

func (s entrySource) String(i int) string { return s[i].text() }
func (s entrySource) Len() int            { return len(s) }

// match keeps the entries that fuzzy-match filter, best match first; an empty filter keeps all in order.
func match(entries []Entry, filter string) []Entry {
	if filter == "" {
		return entries
	}
	var out []Entry
	for _, m := range fuzzy.FindFrom(filter, entrySource(entries)) {
		out = append(out, entries[m.Index])
	}
	return out
}

// ProjectLister fetches an owner's projects; gh.Client.OwnerProjects in production.
type ProjectLister func(owner string) ([]gh.ProjectRef, error)

// PickedMsg is sent when a project was chosen in the picker.
type PickedMsg struct{ Ref gh.ProjectRef }

type ownerProjectsMsg struct {
	owner string
	refs  []gh.ProjectRef
	err   error
}

// Picker lets the user choose a project. Keys are fixed (shortcut: not rebindable, typing filters,
// so letters like q/j/k cannot be actions; revisit with the ctrl+p switcher).
type Picker struct {
	theme         theme.Theme
	list          ProjectLister
	pages         []Page
	width, height int
	busy          string
	err           error
}

func NewPicker(th theme.Theme, list ProjectLister, first Page) Picker {
	return Picker{theme: th, list: list, pages: []Page{first}, width: 80, height: 24}
}

func (p Picker) page() *Page { return &p.pages[len(p.pages)-1] }

func (p Picker) Update(msg tea.Msg) (Picker, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
	case ownerProjectsMsg:
		p.busy = ""
		if msg.err != nil {
			p.err = msg.err
			break
		}
		entries := ProjectEntries(msg.refs)
		sub := fmt.Sprintf("%d open projects", len(entries))
		p.pages = append(p.pages, Page{Title: "Projects of " + msg.owner, Subtitle: sub, Entries: entries})
	case tea.KeyPressMsg:
		if p.busy != "" && msg.String() != "ctrl+c" {
			break
		}
		pg := p.page()
		vis := pg.visible()
		switch msg.String() {
		case "ctrl+c":
			return p, tea.Quit
		case "up", "ctrl+k":
			pg.cursor--
		case "down", "ctrl+j":
			pg.cursor++
		case "esc":
			switch {
			case pg.filter != "":
				pg.filter, pg.cursor = "", 0
			case len(p.pages) > 1:
				p.pages = p.pages[:len(p.pages)-1]
			}
			p.err = nil
		case "backspace":
			if r := []rune(pg.filter); len(r) > 0 {
				pg.filter, pg.cursor = string(r[:len(r)-1]), 0
			}
		case "enter":
			if pg.cursor >= len(vis) {
				break
			}
			e := vis[pg.cursor]
			p.err = nil
			if e.Ref != nil {
				ref := *e.Ref
				return p, func() tea.Msg { return PickedMsg{ref} }
			}
			p.busy = "Loading projects of " + e.Owner + "…"
			owner, list := e.Owner, p.list
			return p, func() tea.Msg {
				refs, err := list(owner)
				return ownerProjectsMsg{owner, refs, err}
			}
		default:
			if t := msg.Text; t != "" {
				pg.filter, pg.cursor = pg.filter+t, 0
			}
		}
		p.clamp()
	}
	return p, nil
}

// entryHeight: projects are two-line cards, owners one line.
func (pg *Page) entryHeight() int {
	for _, e := range pg.Entries {
		if e.Ref != nil {
			return 2
		}
	}
	return 1
}

func (p Picker) listHeight() int { return max(1, (p.height-6)/p.page().entryHeight()) }

func (p *Picker) clamp() {
	pg := p.page()
	pg.cursor = max(0, min(pg.cursor, len(pg.visible())-1))
	n := p.listHeight()
	pg.offset = max(min(pg.offset, pg.cursor), pg.cursor-n+1, 0)
}

func (p Picker) View() tea.View {
	t := p.theme
	pg := p.page()
	lines := []string{
		t.Fg("accent").Bold(true).Render(" " + pg.Title),
		t.Fg("muted").Render(" " + pg.Subtitle),
		" > " + pg.filter + "▌",
		"",
	}
	lines = append(lines, renderEntries(t, pg.visible(), pg.cursor, pg.offset, p.listHeight())...)
	for len(lines) < p.height-2 {
		lines = append(lines, "")
	}
	status := t.Fg("muted").Render(" ↑↓ move · enter open · type to filter · esc back · ctrl+c quit")
	switch {
	case p.busy != "":
		status = " " + p.busy
	case p.err != nil:
		status = t.Fg("danger").Render(" " + p.err.Error())
	}
	lines = append(lines, t.Fg("border").Render(strings.Repeat("─", p.width)), status)
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, p.width, "…")
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	return v
}

// renderEntries draws n entries from offset: owners as one line, projects as two-line cards.
func renderEntries(t theme.Theme, vis []Entry, cursor, offset, n int) []string {
	var lines []string
	for i := offset; i < min(len(vis), offset+n); i++ {
		e, mark := vis[i], "  "
		if i == cursor {
			mark = t.Fg("accent").Render("▌ ")
		}
		if e.Ref == nil {
			lines = append(lines, " "+mark+e.Owner)
			continue
		}
		lines = append(lines, " "+mark+t.Fg("accent").Bold(i == cursor).Render(e.Ref.Title),
			" "+mark+t.Fg("muted").Render(fmt.Sprintf("%s/#%d", e.Ref.Owner, e.Ref.Number)))
	}
	if len(vis) == 0 {
		lines = append(lines, t.Fg("muted").Render("   nothing matches"))
	}
	return lines
}
