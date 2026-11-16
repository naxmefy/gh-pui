package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/naxmefy/gh-pui/internal/config"
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
	col       int                     // focused column of the cursor row
	edit      *editor                 // open edit popup
	save      SaveFunc                // nil: read-only
	status    string                  // one-shot message in place of the help line
	statusErr bool
	settings  config.ProjectSettings // local view options from projects.yml
	slice     map[int]int            // view -> selected slice value (0 = all)
	expanded  map[string]bool        // item Ref -> sub-issues shown
	move      MoveFunc               // nil: board moves change the column only
	board     boardState
}

// WithSettings applies projects.yml settings the API cannot provide (sums, hierarchy, slice).
func (m Model) WithSettings(s config.ProjectSettings) Model {
	m.settings = s
	return m
}

func (m Model) viewSettings() config.ViewSettings { return m.settings.Views[m.currentView().Name] }

func (m Model) hierarchy() bool {
	h := m.viewSettings().Hierarchy
	return h == nil || *h
}

// sliceState returns the slice field, its values ("" = items without a value) and the selected index (0 = all).
func (m Model) sliceState() (field string, values []string, sel int) {
	field = m.viewSettings().Slice
	if field == "" || m.isBoard() { // shortcut: no slice panel on boards yet
		return "", nil, 0
	}
	values = sliceValues(m.project, field, viewItems(m.project, m.currentView()))
	return field, values, min(m.slice[m.view], len(values))
}

// WithSave enables editing; without it the table is read-only.
func (m Model) WithSave(save SaveFunc) Model {
	m.save = save
	return m
}

func (m Model) editing() bool { return m.edit != nil }

// WithMove lets board moves keep the dropped position; without it only the column changes.
func (m Model) WithMove(move MoveFunc) Model {
	m.move = move
	return m
}

// item finds an item by id; pointers into project.Items go stale when a board move reorders them.
func (m Model) item(id string) *project.Item {
	for i := range m.project.Items {
		if m.project.Items[i].ID == id {
			return &m.project.Items[i]
		}
	}
	return nil
}

func NewModel(login string, p *project.Project, th theme.Theme, keys KeyMap) Model {
	return Model{login: login, project: p, theme: th, keys: keys, width: 80, height: 24, bp: layout.For(80),
		collapsed: map[int]map[string]bool{}, slice: map[int]int{}, expanded: map[string]bool{}}
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
	items := viewItems(m.project, v)
	if field, values, sel := m.sliceState(); sel > 0 {
		items = sliceItems(items, field, values[sel-1])
	}
	// show hierarchy decides once which items are counted: groups hold only the top-level ones
	var all []*project.Item
	if m.hierarchy() {
		all, items = items, roots(items)
	}
	groups = groupItems(m.project, v, items, sumFields(m.settings.SumFor(v.Name)))
	return groups, rows(groups, grouped, m.collapsed[m.view], all, m.expanded), grouped
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height, m.bp = msg.Width, msg.Height, layout.For(msg.Width)
	case savedMsg:
		m.status, m.statusErr = "", false
		if it := m.item(msg.id); msg.err != nil && it != nil {
			it.Values[msg.field] = msg.old
			m.status, m.statusErr = fmt.Sprintf("%s not saved: %v", msg.field, msg.err), true
		}
	case movedMsg:
		m.status, m.statusErr = "", false
		if msg.err != nil {
			m.rollback(msg)
		}
	case tea.KeyPressMsg:
		if m.edit != nil {
			return m.updateEdit(msg)
		}
		m.status, m.statusErr = "", false
		if m.isBoard() {
			return m.updateBoard(msg)
		}
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
		case key.Matches(msg, m.keys.Left):
			m.col--
		case key.Matches(msg, m.keys.Right):
			m.col++
		case key.Matches(msg, m.keys.Edit) && len(rs) > 0:
			col := ""
			if m.col > 0 {
				cols, _ := m.visibleCols(rs)
				col = cols[m.col]
			}
			m.startEdit(rs[m.cursor].item, col)
		case key.Matches(msg, m.keys.Toggle) && len(rs) > 0:
			m.toggle(rs, grouped)
		case key.Matches(msg, m.keys.SliceNext, m.keys.SlicePrev):
			if _, values, sel := m.sliceState(); values != nil {
				step := 1
				if key.Matches(msg, m.keys.SlicePrev) {
					step = len(values)
				}
				m.slice[m.view], m.cursor, m.offset = (sel+step)%(len(values)+1), 0, 0
			}
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

// startEdit opens the editor for the field col of an item; without col (the title column, a board card)
// it asks which field to edit, which also reaches fields that are hidden or used for grouping (like Status).
func (m *Model) startEdit(it *project.Item, col string) {
	switch {
	case m.save == nil:
		m.status = "read-only"
	case it == nil:
		m.status = "move to an item to edit it"
	case it.Type == "REDACTED":
		m.status = "this item is not accessible"
	case col == "":
		m.edit = newChooser(m.project, it)
	default:
		name := col
		for _, f := range m.project.Fields {
			if f.Name == name && editable(f) {
				m.edit = newEditor(it, f)
				return
			}
		}
		m.status = name + " cannot be edited here"
	}
}

// updateEdit routes keys to the open editor and saves optimistically: the table shows the new value at once,
// savedMsg rolls it back if GitHub refuses.
func (m Model) updateEdit(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	e := m.edit
	cmd, done, apply := e.update(msg)
	if !done {
		return m, cmd
	}
	if !apply {
		m.edit = nil
		return m, nil
	}
	v, val, err := e.result()
	if err != nil {
		e.err = err.Error()
		return m, nil
	}
	m.edit = nil
	old := e.item.Values[e.field.Name]
	e.item.Values[e.field.Name] = val
	m.status = "saving " + e.field.Name + "…"
	save, pid, id, field := m.save, m.project.ID, e.item.ID, e.field
	m.clamp()
	return m, func() tea.Msg {
		return savedMsg{id: id, field: field.Name, old: old, err: save(pid, id, field.ID, v)}
	}
}

// toggle folds or unfolds the group of the current row and keeps the cursor on its header.
// On a parent issue it expands or collapses its sub-issues instead; on a sub-issue it collapses
// the parent and moves the cursor there.
func (m *Model) toggle(rs []row, grouped bool) {
	r := rs[m.cursor]
	if r.item != nil && r.children > 0 {
		m.expanded[r.item.Ref()] = !m.expanded[r.item.Ref()]
		return
	}
	if r.item != nil && r.depth > 0 {
		for i := m.cursor - 1; i >= 0; i-- {
			if rs[i].item != nil && rs[i].depth == r.depth-1 {
				m.expanded[rs[i].item.Ref()] = false
				m.cursor = i
				return
			}
		}
	}
	if !grouped {
		return
	}
	groups, _, _ := m.table()
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
	m.view, m.cursor, m.offset, m.board = (i%n+n)%n, 0, 0, boardState{}
}

// clamp keeps the cursor inside the rows and the scroll offset around the cursor; rows can be two lines high (xs cards).
func (m *Model) clamp() {
	if m.isBoard() {
		m.clampBoard()
		return
	}
	_, rs, _ := m.table()
	m.cursor = max(0, min(m.cursor, len(rs)-1))
	cols, _ := m.visibleCols(rs)
	m.col = max(0, min(m.col, len(cols)-1))
	body := m.bodyHeight()
	m.offset = max(0, min(m.offset, m.cursor))
	for m.offset < m.cursor && m.linesBetween(rs, m.offset, m.cursor+1) > body {
		m.offset++
	}
	for m.offset > 0 && m.linesBetween(rs, m.offset-1, len(rs)) <= body { // no empty space below the last row
		m.offset--
	}
}

func (m Model) rowLines(r row) int {
	if r.item != nil && m.bp == layout.XS {
		return 2
	}
	return 1
}

func (m Model) linesBetween(rs []row, from, to int) int {
	n := 0
	for _, r := range rs[from:to] {
		n += m.rowLines(r)
	}
	return n
}

const peekHeight = 4

func (m Model) showPeek() bool { return m.height >= 20 && m.bp > layout.XS }

// chromeHeight: header, tabs, column header and statusbar; xs folds the tabs into the header and has no column header.
func (m Model) chromeHeight() int {
	if m.bp == layout.XS {
		return 2
	}
	return 4
}

// bodyHeight is the number of lines left for table rows.
func (m Model) bodyHeight() int {
	h := m.height - m.chromeHeight()
	if m.showPeek() {
		h -= peekHeight
	}
	return max(1, h)
}

func (m Model) View() tea.View {
	if m.isBoard() {
		return m.finish(strings.Join(m.boardLines(), "\n"))
	}
	groups, rs, _ := m.table()
	cols, widths := m.visibleCols(rs)

	bm := m.bodyModel()
	var table []string
	if m.bp > layout.XS {
		table = append(table, bm.renderColumns(cols, widths))
	}
	body := m.bodyHeight()
	for i, used := m.offset, 0; i < len(rs) && used+m.rowLines(rs[i]) <= body; i++ {
		table = append(table, bm.renderRow(rs[i], groups, cols, widths, i == m.cursor)...)
		used += m.rowLines(rs[i])
	}
	if len(rs) == 0 {
		table = append(table, m.theme.Fg("muted").Render(" No items in this view"))
	}
	want := body
	if m.bp > layout.XS {
		want++ // column header
	}
	for len(table) < want {
		table = append(table, "")
	}
	if m.showSlicePanel() {
		panel := m.renderSlicePanel(len(table))
		for i := range table {
			table[i] = panel[i] + table[i]
		}
	}
	lines := []string{m.renderTop()}
	if m.bp > layout.XS {
		lines = append(lines, m.renderTabs())
	}
	lines = append(lines, table...)
	if m.showPeek() {
		lines = append(lines, m.renderPeek(rs)...)
	}
	lines = append(lines, m.renderStatus(rs))

	return m.finish(strings.Join(lines, "\n"))
}

// finish puts the edit popup over the screen.
func (m Model) finish(content string) tea.View {
	if m.edit != nil {
		content = overlay(m.theme, content, m.edit.lines(m.theme), max(20, min(m.width-4, 50)), m.width, m.height)
	}
	out := tea.NewView(content)
	out.AltScreen = true
	return out
}

// spread puts left and right on one line of the full width, cutting left when both do not fit.
func (m Model) spread(left, right string) string {
	left = ansi.Truncate(left, max(0, m.width-ansi.StringWidth(right)-1), "…")
	return left + strings.Repeat(" ", max(1, m.width-ansi.StringWidth(left)-ansi.StringWidth(right))) + right
}

func (m Model) layoutGlyph(v project.View) string {
	switch v.Layout {
	case "BOARD_LAYOUT":
		return m.theme.Symbol("layout.board")
	case "ROADMAP_LAYOUT":
		return m.theme.Symbol("layout.roadmap")
	}
	return m.theme.Symbol("layout.table")
}

// renderTop is the breadcrumb bar; it drops the owner and item count as the width shrinks.
func (m Model) renderTop() string {
	t, p := m.theme, m.project
	sep := t.Fg("muted").Render(" " + t.Symbol("sep") + " ")
	root := t.Fg("accent").Bold(true).Render("gh pui")
	title := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("#%d %s", p.Number, p.Title))
	right := t.Fg("muted").Render("@" + m.login + " ")
	var left string
	switch {
	case m.bp == layout.XS:
		left = " " + title
		if n := len(p.Views); n > 0 {
			right = t.Badge("accent").Render(fmt.Sprintf(" %s %d/%d ", m.layoutGlyph(m.currentView()), m.view+1, n))
		}
	case m.bp <= layout.MD:
		left = " " + root + sep + title
	default:
		items := 0
		for _, it := range p.Items {
			if !it.Archived {
				items++
			}
		}
		left = " " + root + sep + t.Fg("muted").Render(p.Owner) + sep + title + t.Fg("muted").Render(fmt.Sprintf("  %d items", items))
	}
	return m.spread(left, right)
}

// renderTabs shows every view as `n glyph name`; on sm only the active tab keeps its name.
func (m Model) renderTabs() string {
	t := m.theme
	var parts []string
	for i, v := range m.project.Views {
		label := fmt.Sprintf("%d %s %s", i+1, m.layoutGlyph(v), v.Name)
		if i == m.view {
			parts = append(parts, t.Badge("accent").Render(" "+label+" "))
			continue
		}
		if m.bp == layout.SM {
			label = fmt.Sprintf("%d%s", i+1, m.layoutGlyph(v))
		}
		parts = append(parts, t.Fg("muted").Render(label))
	}
	left := " " + strings.Join(parts, "  ")
	if field, values, sel := m.sliceState(); field != "" && !m.showSlicePanel() { // sm: the slice panel does not fit
		value := "All"
		if sel > 0 {
			value = values[sel-1]
			if value == "" {
				value = "No " + field
			}
		}
		return m.spread(left, t.Fg("muted").Render(field+": ")+value+" ")
	}
	return ansi.Truncate(left, m.width, "…")
}

// indent marks the hierarchy before a title: sub-issues get `└` under their parent,
// parents a fold marker (▸ collapsed, ▾ expanded).
func (m Model) indent(r row) (plain, styled string) {
	if r.depth > 0 {
		plain = strings.Repeat("  ", r.depth-1) + m.theme.Symbol("child") + " "
	}
	if r.children > 0 {
		fold := m.theme.Symbol("collapsed")
		if m.expanded[r.item.Ref()] {
			fold = m.theme.Symbol("expanded")
		}
		plain += fold + " "
	}
	return plain, m.theme.Fg("muted").Render(plain)
}

// prefix is the muted part before a title: repo#number, or "draft".
func prefix(it *project.Item) string {
	switch {
	case it.Type == "DRAFT_ISSUE":
		return "draft"
	case it.Number == 0:
		return ""
	case it.Repo != "":
		_, name, _ := strings.Cut(it.Repo, "/")
		return fmt.Sprintf("%s#%d", name, it.Number)
	}
	return fmt.Sprintf("#%d", it.Number)
}

// optionRole is the color role of a single-select value, e.g. "option.green" for a green Status option.
func (m Model) optionRole(field, value string) string {
	for _, f := range m.project.Fields {
		if f.Name != field || f.DataType != "SINGLE_SELECT" {
			continue
		}
		for _, o := range f.Options {
			if o.Name == value && o.Color != "" {
				return "option." + strings.ToLower(o.Color)
			}
		}
	}
	return ""
}

// cell returns a cell's plain text (for widths and the inverted cursor row) and its styled form.
func (m Model) cell(it *project.Item, col string) (plain, styled string) {
	t := m.theme
	if col == "Title" {
		pre := prefix(it)
		if pre == "" {
			return it.Title, it.Title
		}
		return pre + " " + it.Title, t.Fg("muted").Render(pre) + " " + it.Title
	}
	v := it.Values[col].Text
	if v == "" {
		return t.Symbol("empty"), t.Fg("muted").Render(t.Symbol("empty"))
	}
	if role := m.optionRole(col, v); role != "" {
		dot := t.Symbol("dot")
		return dot + " " + v, t.Fg(role).Render(dot) + " " + v
	}
	return v, v
}

// visibleCols are the view's columns that fit the width, with their widths.
func (m Model) visibleCols(rs []row) ([]string, []int) {
	bm := m.bodyModel()
	cols := columns(m.currentView(), m.bp)
	widths := bm.columnWidths(cols, rs)
	return cols[:len(widths)], widths
}

const slicePanelWidth = 24

func (m Model) showSlicePanel() bool {
	field, _, _ := m.sliceState()
	return field != "" && m.bp >= layout.MD
}

// bodyModel is the model the table body renders with: narrower by the slice panel when it is shown.
func (m Model) bodyModel() Model {
	if m.showSlicePanel() {
		m.width -= slicePanelWidth
	}
	return m
}

// renderSlicePanel lists the slice field's values with counts; the first line sits next to the column header.
func (m Model) renderSlicePanel(height int) []string {
	t := m.theme
	field, values, sel := m.sliceState()
	items := viewItems(m.project, m.currentView())
	count := func(its []*project.Item) int { // same rule as the group count
		if m.hierarchy() {
			its = roots(its)
		}
		n, _ := tally(its, nil)
		return n
	}
	labels := []string{"All"}
	counts := []int{count(items)}
	for _, v := range values {
		label := v
		if v == "" {
			label = "No " + field
		}
		labels, counts = append(labels, label), append(counts, count(sliceItems(items, field, v)))
	}
	w := slicePanelWidth - 2
	lines := []string{t.Fg("muted").Render(pad(" "+strings.ToUpper(field), w))}
	for i, l := range labels {
		n := strconv.Itoa(counts[i])
		text := pad(" "+l, w-len(n)-1) + " " + n
		if i == sel {
			lines = append(lines, t.Fg("accent").Render(t.Symbol("bar"))+lipgloss.NewStyle().Bold(true).Render(ansi.Truncate(text[1:], w-1, "")))
		} else {
			lines = append(lines, text)
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, l := range lines[:height] {
		lines[i] = pad(l, w) + " " + t.Fg("border").Render("│")
	}
	return lines[:height]
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
				plain, _ := m.cell(r.item, c)
				w = max(w, ansi.StringWidth(plain))
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

func (m Model) renderColumns(cols []string, widths []int) string {
	cells := []string{"   " + pad(strings.ToUpper(cols[0]), widths[0])}
	for i, c := range cols[1:] {
		cells = append(cells, pad(strings.ToUpper(c), widths[i+1]))
	}
	return ansi.Truncate(m.theme.Fg("muted").Render(strings.Join(cells, "  ")), m.width, "")
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

// renderRow draws a group header or an item: one line, or a two-line card on xs.
func (m Model) renderRow(r row, groups []group, cols []string, widths []int, selected bool) []string {
	t := m.theme
	if r.item == nil {
		g := groups[r.group]
		fold := t.Symbol("expanded")
		if m.collapsed[m.view][g.Key] {
			fold = t.Symbol("collapsed")
		}
		dot := ""
		if v := m.currentView(); len(v.GroupBy) > 0 {
			if role := m.optionRole(v.GroupBy[0], g.Key); role != "" {
				dot = t.Fg(role).Render(t.Symbol("dot")) + " "
			}
		}
		var meta []string
		if m.settings.CountFor(m.currentView().Name) {
			switch {
			case m.bp == layout.XS:
				meta = append(meta, strconv.Itoa(g.Count))
			case g.Count == 1:
				meta = append(meta, "1 item")
			default:
				meta = append(meta, fmt.Sprintf("%d items", g.Count))
			}
		}
		for _, s := range g.Sums {
			meta = append(meta, "Σ "+s.Field+" "+strconv.FormatFloat(s.Value, 'f', -1, 64))
		}
		line := " " + fold + " " + dot + lipgloss.NewStyle().Bold(true).Render(g.Label) + "  " + t.Fg("muted").Render(strings.Join(meta, " · "))
		if selected {
			return []string{t.Selected().Render(pad(ansi.Strip(line), m.width))}
		}
		return []string{ansi.Truncate(line, m.width, "…")}
	}

	it := r.item
	lead := " "
	if selected {
		lead = t.Fg("accent").Render(t.Symbol("bar"))
	}
	if m.bp == layout.XS {
		return m.renderCard(r, lead, selected)
	}
	if selected {
		// the cursor row is inverted (or the "selected" background); a focused field cell stands out un-inverted and underlined
		sel, focus := t.Selected(), lipgloss.NewStyle().Bold(true).Underline(true)
		var b strings.Builder
		b.WriteString(lead + sel.Render(ansi.Strip(m.symbol(it))+" "))
		for i, c := range cols {
			if i > 0 {
				b.WriteString(sel.Render("  "))
			}
			st := sel
			if i == m.col && i > 0 { // the title column is the default, it needs no extra mark
				st = focus
			}
			plain, _ := m.cell(it, c)
			if i == 0 {
				in, _ := m.indent(r)
				plain = in + plain
			}
			b.WriteString(st.Render(pad(plain, widths[i])))
		}
		b.WriteString(sel.Render(strings.Repeat(" ", max(0, m.width-ansi.StringWidth(b.String())))))
		return []string{ansi.Truncate(b.String(), m.width, "")}
	}
	cells := make([]string, len(cols))
	for i, c := range cols {
		_, styled := m.cell(it, c)
		if i == 0 {
			_, in := m.indent(r)
			styled = in + styled
		}
		cells[i] = pad(styled, widths[i])
	}
	return []string{ansi.Truncate(lead+m.symbol(it)+" "+strings.Join(cells, "  "), m.width, "…")}
}

// renderCard is the xs form of an item: `symbol repo#num` with a few field values on the right, then the title.
func (m Model) renderCard(r row, lead string, selected bool) []string {
	t, it := m.theme, r.item
	var vals []string
	for _, c := range columns(m.currentView(), layout.MD)[1:] {
		if v := it.Values[c].Text; v != "" && len(vals) < 2 {
			vals = append(vals, v)
		}
	}
	_, in := m.indent(r)
	first := in + m.symbol(it) + " " + t.Fg("muted").Render(prefix(it))
	right := strings.Join(vals, " ") + " "
	first = ansi.Truncate(first, max(0, m.width-1-ansi.StringWidth(right)-1), "…")
	first += strings.Repeat(" ", max(1, m.width-1-ansi.StringWidth(first)-ansi.StringWidth(right))) + right
	second := ansi.Truncate("  "+it.Title, m.width-1, "…")
	if selected {
		sel := t.Selected()
		return []string{lead + sel.Render(pad(ansi.Strip(first), m.width-1)), " " + sel.Render(pad(second, m.width-1))}
	}
	return []string{lead + first, " " + second}
}

func (m Model) renderPeek(rs []row) []string {
	t := m.theme
	lines := []string{t.Fg("accent").Render(strings.Repeat("─", m.width))}
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

// renderStatus is the vim/tmux-like bottom bar: mode badge, key hints (or a message), position on the right.
func (m Model) renderStatus(rs []row) string {
	t := m.theme
	mode := "TABLE"
	if m.bp == layout.XS {
		mode = "TBL"
	}
	left := t.Badge("accent").Render(" " + mode + " ")
	if m.edit != nil {
		left += t.Badge("muted").Render(" EDIT · " + m.editTitle() + " ")
	}

	items, at := 0, 0
	for i, r := range rs {
		if r.item != nil {
			items++
			if i <= m.cursor {
				at = items
			}
		}
	}
	pos := fmt.Sprintf("%d/%d", at, items)
	if len(rs) > 0 && rs[m.cursor].item == nil {
		pos = fmt.Sprintf("–/%d", items)
	}
	right := pos + " "
	if m.bp > layout.SM {
		cols, _ := m.visibleCols(rs)
		right = fmt.Sprintf("row %s · col %d/%d ", pos, m.col+1, len(cols))
	}
	right = t.Fg("muted").Render(right)

	var hints string
	switch {
	case m.status != "":
		st := t.Fg("muted")
		if m.statusErr {
			st = t.Fg("danger")
		}
		hints = st.Render(m.status)
	case m.edit != nil:
		hints = hint(t, "enter", "save") + "  " + hint(t, "esc", "cancel")
	default:
		k := m.keys
		pairs := [][2]string{
			{k.Down.Help().Key + "/" + k.Up.Help().Key, "move"},
			{k.Left.Help().Key + "/" + k.Right.Help().Key, "column"},
			{k.Edit.Help().Key, "edit"},
			{k.Toggle.Help().Key, "fold"},
			{k.NextView.Help().Key, "view"},
			{k.Switch.Help().Key, "projects"},
			{k.Quit.Help().Key, "quit"},
		}
		if field, _, _ := m.sliceState(); field != "" {
			pairs = append(pairs[:5:5], append([][2]string{{k.SlicePrev.Help().Key + "/" + k.SliceNext.Help().Key, "slice"}}, pairs[5:]...)...)
		}
		room := m.width - ansi.StringWidth(left) - ansi.StringWidth(right) - 3
		for _, p := range pairs { // whole hints only, as many as fit
			h := hint(t, p[0], p[1])
			if hints != "" {
				h = "  " + h
			}
			if ansi.StringWidth(hints+h) > room {
				break
			}
			hints += h
		}
	}
	room := m.width - ansi.StringWidth(left) - ansi.StringWidth(right) - 3
	return m.spread(left+"  "+ansi.Truncate(hints, max(0, room), "…"), right)
}

func hint(t theme.Theme, key, desc string) string {
	return lipgloss.NewStyle().Bold(true).Render(key) + " " + t.Fg("muted").Render(desc)
}

func (m Model) editTitle() string {
	if m.edit.kind == chooseField {
		return "field"
	}
	return m.edit.field.Name
}
