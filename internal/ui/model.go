package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/layout"
	"github.com/naxmefy/gh-pui/internal/ui/theme"
)

type Model struct {
	login     string
	project   *project.Project
	theme     theme.Theme
	keys      KeyMap
	width     int
	height    int
	bp        layout.Breakpoint
	view      int                     // index into project.Views
	cursor    int                     // index into the current rows
	offset    int                     // first visible row
	collapsed map[int]map[string]bool // view -> group key -> folded
}

func NewModel(login string, p *project.Project, th theme.Theme, keys KeyMap) Model {
	return Model{login: login, project: p, theme: th, keys: keys, width: 80, height: 24, bp: layout.For(80),
		collapsed: map[int]map[string]bool{}}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) currentView() project.View {
	if len(m.project.Views) == 0 {
		return project.View{Name: "All"}
	}
	return m.project.Views[m.view]
}

func (m Model) table() (groups []group, rs []row, grouped bool) {
	v := m.currentView()
	grouped = len(v.GroupBy) > 0
	groups = groupItems(m.project, v, viewItems(m.project, v))
	return groups, rows(groups, grouped, m.collapsed[m.view]), grouped
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height, m.bp = msg.Width, msg.Height, layout.For(msg.Width)
	case tea.KeyPressMsg:
		_, rs, grouped := m.table()
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Down):
			m.cursor++
		case key.Matches(msg, m.keys.Up):
			m.cursor--
		case key.Matches(msg, m.keys.Top):
			m.cursor = 0
		case key.Matches(msg, m.keys.Bottom):
			m.cursor = len(rs) - 1
		case key.Matches(msg, m.keys.Toggle) && grouped && len(rs) > 0:
			m.toggle(rs)
		case key.Matches(msg, m.keys.NextView):
			m.selectView(m.view + 1)
		case key.Matches(msg, m.keys.PrevView):
			m.selectView(m.view - 1)
		default:
			if n, err := strconv.Atoi(msg.String()); err == nil && n >= 1 && n <= len(m.project.Views) {
				m.selectView(n - 1)
			}
		}
	}
	m.clamp()
	return m, nil
}

// toggle folds or unfolds the group of the current row and keeps the cursor on its header.
func (m *Model) toggle(rs []row) {
	groups, _, _ := m.table()
	r := rs[m.cursor]
	k := groups[r.group].Key
	if m.collapsed[m.view] == nil {
		m.collapsed[m.view] = map[string]bool{}
	}
	m.collapsed[m.view][k] = !m.collapsed[m.view][k]
	_, rs, _ = m.table()
	for i, nr := range rs {
		if nr.item == nil && nr.group == r.group {
			m.cursor = i
		}
	}
}

func (m *Model) selectView(i int) {
	n := len(m.project.Views)
	if n == 0 {
		return
	}
	m.view, m.cursor, m.offset = (i%n+n)%n, 0, 0
}

// clamp keeps the cursor inside the rows and the scroll offset around the cursor.
func (m *Model) clamp() {
	_, rs, _ := m.table()
	m.cursor = max(0, min(m.cursor, len(rs)-1))
	visible := m.bodyHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
	m.offset = max(0, min(m.offset, len(rs)-visible))
}

const peekHeight = 4

func (m Model) showPeek() bool { return m.height >= 20 }

// bodyHeight is the number of table rows that fit: total minus tabs, column header, help and peek.
func (m Model) bodyHeight() int {
	h := m.height - 3
	if m.showPeek() {
		h -= peekHeight
	}
	return max(1, h)
}

func (m Model) View() tea.View {
	groups, rs, _ := m.table()
	cols := columns(m.currentView(), m.bp)
	widths := m.columnWidths(cols, rs)
	cols = cols[:len(widths)]

	lines := []string{m.renderTabs(), m.renderHeader(cols, widths)}
	end := min(len(rs), m.offset+m.bodyHeight())
	for i := m.offset; i < end; i++ {
		lines = append(lines, m.renderRow(rs[i], groups, cols, widths, i == m.cursor))
	}
	if len(rs) == 0 {
		lines = append(lines, m.theme.Fg("muted").Render(" No items in this view"))
	}
	for len(lines) < 2+m.bodyHeight() {
		lines = append(lines, "")
	}
	if m.showPeek() {
		lines = append(lines, m.renderPeek(rs)...)
	}
	lines = append(lines, m.renderHelp())

	out := tea.NewView(strings.Join(lines, "\n"))
	out.AltScreen = true
	return out
}

func (m Model) renderTabs() string {
	t := m.theme
	parts := []string{t.Fg("accent").Bold(true).Render(fmt.Sprintf(" %s/%d %s", m.project.Owner, m.project.Number, m.project.Title))}
	for i, v := range m.project.Views {
		label := v.Name
		if m.bp <= layout.SM { // narrow: tabs become their number shortcuts
			label = strconv.Itoa(i + 1)
		}
		if i == m.view {
			parts = append(parts, t.Fg("accent").Underline(true).Render(label))
		} else {
			parts = append(parts, t.Fg("muted").Render(label))
		}
	}
	return ansi.Truncate(strings.Join(parts, "  "), m.width, "…")
}

func cellText(it *project.Item, col string) string {
	if col == "Title" {
		if it.Number > 0 {
			return fmt.Sprintf("%s #%d", it.Title, it.Number)
		}
		return it.Title
	}
	return it.Values[col].Text
}

// columnWidths gives every non-title column its content width (max 24) and the rest to Title,
// dropping trailing columns while Title would get fewer than 20 cells.
func (m Model) columnWidths(cols []string, rs []row) []int {
	const gap, maxCol, minTitle, symbol = 2, 24, 20, 3
	widths := make([]int, len(cols))
	for ci, c := range cols[1:] {
		w := ansi.StringWidth(c)
		for _, r := range rs {
			if r.item != nil {
				w = max(w, ansi.StringWidth(cellText(r.item, c)))
			}
		}
		widths[ci+1] = min(w, maxCol)
	}
	for {
		rest := m.width - symbol
		for _, w := range widths[1:] {
			rest -= w + gap
		}
		if rest >= minTitle || len(widths) == 1 {
			widths[0] = max(rest, 1)
			return widths
		}
		widths = widths[:len(widths)-1]
	}
}

func pad(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}

func (m Model) renderHeader(cols []string, widths []int) string {
	cells := []string{"   " + pad(cols[0], widths[0])}
	for i, c := range cols[1:] {
		cells = append(cells, pad(c, widths[i+1]))
	}
	return m.theme.Fg("muted").Bold(true).Render(strings.Join(cells, "  "))
}

func (m Model) symbol(it *project.Item) string {
	t := m.theme
	switch {
	case it.Type == "DRAFT_ISSUE":
		return t.Fg("muted").Render(t.Symbol("draft"))
	case it.Type == "PULL_REQUEST" && it.State == "MERGED":
		return t.Fg("status.review").Render(t.Symbol("pr.merged"))
	case it.Type == "PULL_REQUEST":
		return t.Fg("success").Render(t.Symbol("pr"))
	case it.State == "CLOSED":
		return t.Fg("status.done").Render(t.Symbol("issue.closed"))
	}
	return t.Fg("success").Render(t.Symbol("issue"))
}

func (m Model) renderRow(r row, groups []group, cols []string, widths []int, selected bool) string {
	t := m.theme
	var line string
	if r.item == nil {
		g := groups[r.group]
		fold := t.Symbol("expanded")
		if m.collapsed[m.view][g.Key] {
			fold = t.Symbol("collapsed")
		}
		meta := []string{strconv.Itoa(len(g.Items))}
		for _, s := range g.Sums {
			meta = append(meta, s.Field+" "+strconv.FormatFloat(s.Value, 'f', -1, 64))
		}
		line = " " + fold + " " + t.Fg("accent").Bold(true).Render(g.Label) + "  " + t.Fg("muted").Render(strings.Join(meta, " · "))
	} else {
		cells := []string{" " + m.symbol(r.item) + " " + pad(cellText(r.item, cols[0]), widths[0])}
		for i, c := range cols[1:] {
			cells = append(cells, pad(cellText(r.item, c), widths[i+1]))
		}
		line = strings.Join(cells, "  ")
	}
	if selected {
		// inverted selection works on every color profile, including none
		return lipgloss.NewStyle().Reverse(true).Render(pad(ansi.Strip(line), m.width))
	}
	return ansi.Truncate(line, m.width, "…")
}

func (m Model) renderPeek(rs []row) []string {
	t := m.theme
	lines := []string{t.Fg("border").Render(strings.Repeat("─", m.width))}
	if len(rs) == 0 || rs[m.cursor].item == nil {
		return append(lines, "", "", "")
	}
	it := rs[m.cursor].item
	head := []string{strings.ToLower(strings.ReplaceAll(it.Type, "_", " "))}
	if it.Repo != "" {
		head = append(head, fmt.Sprintf("%s#%d", it.Repo, it.Number), strings.ToLower(it.State))
	}
	var fields []string
	for _, f := range []string{"Status", "Assignees", "Labels", "Milestone", "Parent issue", "Sub-issues progress"} {
		if v := it.Values[f].Text; v != "" {
			fields = append(fields, f+": "+v)
		}
	}
	return append(lines,
		ansi.Truncate(" "+t.Fg("muted").Render(strings.Join(head, " · ")), m.width, "…"),
		ansi.Truncate(" "+lipgloss.NewStyle().Bold(true).Render(it.Title), m.width, "…"),
		ansi.Truncate(" "+strings.Join(fields, " · "), m.width, "…"))
}

func (m Model) renderHelp() string {
	var parts []string
	for _, b := range m.keys.help() {
		parts = append(parts, b.Help().Key+" "+b.Help().Desc)
	}
	parts = append(parts, m.bp.String())
	return ansi.Truncate(m.theme.Fg("muted").Render(" "+strings.Join(parts, " · ")), m.width, "…")
}
