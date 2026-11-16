package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/naxmefy/gh-pui/internal/gh"
	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/layout"
)

// boardState is the cursor of a board view: focused column and card, the first column shown,
// and the card being moved (the cursor is then its drop target).
type boardState struct {
	col, card, scroll int
	lifted            *lifted
}

type lifted struct {
	item      *project.Item
	col, card int // where it was lifted, for esc
}

// movedMsg reports a finished drop; on error the local column and position are rolled back.
type movedMsg struct {
	id         string
	field      string // "" when the column did not change
	old        project.Value
	fieldSaved bool
	prevAfter  string // the item before it in the project order, to put it back
	reordered  bool
	err        error
}

const (
	boardColMin  = 32
	foldedWidth  = 3
	boardHeadLen = 2
)

func (m Model) isBoard() bool { return m.currentView().Layout == "BOARD_LAYOUT" }

// boardField is the view's column field; github.com uses Status when none is set.
func (m Model) boardField() string {
	if v := m.currentView(); len(v.VerticalGroupBy) > 0 {
		return v.VerticalGroupBy[0]
	}
	return "Status"
}

// boardColumns has one column per option (or open iteration) of the column field, empty ones included so cards
// can be dropped there; values without an option follow, and "No <field>" comes first when items lack a value,
// like github.com. Boards show every item as a card: no hierarchy, counts and sums over all of them.
func (m Model) boardColumns() []group {
	field, v := m.boardField(), m.currentView()
	groups := groupItems(m.project, project.View{GroupBy: []string{field}}, viewItems(m.project, v), nil) // counts and sums: renderBoardColumn
	have := map[string]group{}
	for _, g := range groups {
		have[g.Key] = g
	}
	var out []group
	if g, ok := have[""]; ok {
		out = append(out, g)
	}
	var order []string
	for _, f := range m.project.Fields {
		if f.Name != field {
			continue
		}
		for _, o := range f.Options {
			order = append(order, o.Name)
		}
		for _, it := range f.Iterations {
			if !it.Completed {
				order = append(order, it.Title)
			}
		}
	}
	for _, k := range order {
		g, ok := have[k]
		if !ok {
			g = group{Key: k, Label: k}
		}
		out = append(out, g)
	}
	for _, g := range groups {
		if g.Key != "" && !contains(order, g.Key) {
			out = append(out, g)
		}
	}
	return out
}

// cards lists each column's cards; a lifted card is taken out of its column and shown at the cursor.
func (m Model) cards(cols []group) [][]*project.Item {
	out := make([][]*project.Item, len(cols))
	lf := m.board.lifted
	for i, c := range cols {
		for _, it := range c.Items {
			if lf == nil || it != lf.item {
				out[i] = append(out[i], it)
			}
		}
	}
	if lf != nil && m.board.col < len(out) {
		list := out[m.board.col]
		at := max(0, min(m.board.card, len(list)))
		out[m.board.col] = append(list[:at:at], append([]*project.Item{lf.item}, list[at:]...)...)
	}
	return out
}

func (m Model) folded(key string) bool { return m.bp > layout.XS && m.collapsed[m.view][key] }

func (m *Model) clampBoard() {
	cols := m.boardColumns()
	b := &m.board
	b.col = max(0, min(b.col, len(cols)-1))
	if len(cols) == 0 {
		return
	}
	n := len(m.cards(cols)[b.col])
	b.card = max(0, min(b.card, n-1))
	b.scroll = max(0, min(b.scroll, b.col))
	for b.scroll < b.col && !m.shows(cols, b.scroll, b.col) {
		b.scroll++
	}
}

func (m Model) shows(cols []group, scroll, col int) bool {
	for _, i := range m.boardFit(cols, scroll) {
		if i == col {
			return true
		}
	}
	return false
}

// boardFit returns the columns shown from scroll on; xs shows the focused column alone.
func (m Model) boardFit(cols []group, scroll int) []int {
	if m.bp == layout.XS {
		return []int{m.board.col}
	}
	var out []int
	used := 0
	for i := scroll; i < len(cols); i++ {
		w := boardColMin
		if m.folded(cols[i].Key) {
			w = foldedWidth
		}
		if len(out) > 0 {
			w++ // gap
		}
		if used+w > m.width && len(out) > 0 {
			break
		}
		out, used = append(out, i), used+w
	}
	return out
}

func (m Model) updateBoard(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	cols := m.boardColumns()
	b, k := &m.board, m.keys
	if b.lifted != nil {
		switch {
		case key.Matches(msg, k.Cancel):
			b.col, b.card, b.lifted = b.lifted.col, b.lifted.card, nil
		case key.Matches(msg, k.Toggle, k.Move):
			return m.drop(cols)
		case key.Matches(msg, k.Left, k.Right):
			step := 1
			if key.Matches(msg, k.Left) {
				step = -1
			}
			for i := b.col + step; i >= 0 && i < len(cols); i += step { // folded columns take no drops
				if !m.folded(cols[i].Key) {
					b.col = i
					break
				}
			}
		case key.Matches(msg, k.Up, k.Down, k.Top, k.Bottom):
			if s := m.currentView().SortBy; len(s) > 0 {
				m.status = "sorted by " + s[0].Field + ": the position follows the sort"
				break
			}
			switch {
			case key.Matches(msg, k.Up):
				b.card--
			case key.Matches(msg, k.Down):
				b.card++
			case key.Matches(msg, k.Top):
				b.card = 0
			default:
				b.card = 1 << 30 // clamped to the last position
			}
		}
		m.clampBoard()
		return m, nil
	}
	var card *project.Item
	if len(cols) > 0 {
		if cs := m.cards(cols)[b.col]; b.card < len(cs) && !m.folded(cols[b.col].Key) {
			card = cs[b.card]
		}
	}
	switch {
	case key.Matches(msg, k.Quit):
		return m, tea.Quit
	case key.Matches(msg, k.Down):
		b.card++
	case key.Matches(msg, k.Up):
		b.card--
	case key.Matches(msg, k.Top):
		b.card = 0
	case key.Matches(msg, k.Bottom):
		b.card = 1 << 30 // clamped to the last card
	case key.Matches(msg, k.Left):
		b.col--
	case key.Matches(msg, k.Right):
		b.col++
	case key.Matches(msg, k.Toggle) && len(cols) > 0 && m.bp > layout.XS:
		if m.collapsed[m.view] == nil {
			m.collapsed[m.view] = map[string]bool{}
		}
		ck := cols[b.col].Key
		m.collapsed[m.view][ck] = !m.collapsed[m.view][ck]
	case key.Matches(msg, k.Edit) && card != nil:
		m.startEdit(card, "")
	case key.Matches(msg, k.Move):
		switch {
		case card == nil:
			m.status = "no card to move here"
		case m.save == nil:
			m.status = "read-only"
		case card.Type == "REDACTED":
			m.status = "this item is not accessible"
		default:
			b.lifted = &lifted{item: card, col: b.col, card: b.card}
		}
	case key.Matches(msg, k.NextView):
		m.selectView(m.view + 1)
	case key.Matches(msg, k.PrevView):
		m.selectView(m.view - 1)
	default:
		if n, err := strconv.Atoi(msg.String()); err == nil && n >= 1 && n <= len(m.project.Views) {
			m.selectView(n - 1)
		}
	}
	m.clamp()
	return m, nil
}

// drop saves a lifted card optimistically: the new column (the field value) and, in an unsorted view,
// the position after the card above it.
func (m Model) drop(cols []group) (tea.Model, tea.Cmd) {
	b := &m.board
	lf := b.lifted
	list := m.cards(cols)[b.col]
	b.lifted = nil
	it := lf.item
	sorted := len(m.currentView().SortBy) > 0
	moveCol := b.col != lf.col
	reorder := !sorted && (moveCol || b.card != lf.card)
	if !moveCol && !reorder {
		return m, nil
	}
	msg := movedMsg{id: it.ID}
	var fid string
	var fv gh.FieldValue
	if moveCol {
		f, ok := m.fieldByName(m.boardField())
		if !ok || (f.DataType != "SINGLE_SELECT" && f.DataType != "ITERATION") {
			b.col, b.card = lf.col, lf.card
			m.status, m.statusErr = m.boardField()+" cannot be changed here", true
			return m, nil
		}
		target := cols[b.col].Key
		if target != "" {
			for _, o := range f.Options {
				if o.Name == target {
					fv.OptionID = o.ID
				}
			}
			for _, i := range f.Iterations {
				if i.Title == target {
					fv.IterationID = i.ID
				}
			}
		}
		fid, msg.field, msg.old = f.ID, f.Name, it.Values[f.Name]
		if it.Values == nil {
			it.Values = map[string]project.Value{}
		}
		it.Values[f.Name] = project.Value{Text: target}
	}
	afterID := ""
	if b.card > 0 {
		afterID = list[b.card-1].ID
	}
	if reorder {
		msg.prevAfter, msg.reordered = m.before(it.ID), true
		place(m.project.Items, it.ID, afterID)
	}
	m.status = "moving " + cardRef(it) + "…"
	save, move, pid := m.save, m.move, m.project.ID
	m.clamp()
	return m, func() tea.Msg {
		if msg.field != "" {
			if msg.err = save(pid, msg.id, fid, fv); msg.err != nil {
				return msg
			}
			msg.fieldSaved = true
		}
		if msg.reordered && move != nil {
			msg.err = move(pid, msg.id, afterID)
		}
		return msg
	}
}

// rollback undoes the local part of a drop GitHub refused.
func (m *Model) rollback(msg movedMsg) {
	if it := m.item(msg.id); it != nil && msg.field != "" && !msg.fieldSaved {
		it.Values[msg.field] = msg.old
	}
	if msg.reordered {
		place(m.project.Items, msg.id, msg.prevAfter)
	}
	m.status, m.statusErr = "move not saved: "+msg.err.Error(), true
}

func (m Model) fieldByName(name string) (project.Field, bool) {
	for _, f := range m.project.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return project.Field{}, false
}

// before is the id of the item right before id in the project order ("" for the first).
func (m Model) before(id string) string {
	for i, it := range m.project.Items {
		if it.ID == id && i > 0 {
			return m.project.Items[i-1].ID
		}
	}
	return ""
}

// place moves item id right after afterID ("" = to the front), in place, like updateProjectV2ItemPosition.
func place(items []project.Item, id, afterID string) {
	out := make([]project.Item, 0, len(items))
	var moved *project.Item
	for i := range items {
		if items[i].ID == id {
			moved = &items[i]
			continue
		}
		out = append(out, items[i])
	}
	if moved == nil {
		return
	}
	at := 0
	for i := range out {
		if out[i].ID == afterID {
			at = i + 1
		}
	}
	out = append(out[:at], append([]project.Item{*moved}, out[at:]...)...)
	copy(items, out)
}

func cardRef(it *project.Item) string {
	if p := prefix(it); p != "" {
		return p
	}
	return it.Title
}

// boardLines is the whole board screen: header, tabs, columns of cards, statusbar.
func (m Model) boardLines() []string {
	lines := []string{m.renderTop()}
	if m.bp > layout.XS {
		lines = append(lines, m.renderTabs())
	}
	height := max(boardHeadLen+1, m.height-len(lines)-1)
	cols := m.boardColumns()
	if len(cols) == 0 {
		lines = append(lines, m.theme.Fg("muted").Render(" No "+m.boardField()+" field for the board columns"))
		for len(lines) < m.height-1 {
			lines = append(lines, "")
		}
		return append(lines, m.renderBoardStatus(cols))
	}
	cards := m.cards(cols)
	shown := m.boardFit(cols, m.board.scroll)
	widths := make([]int, len(shown))
	rest, open := m.width-(len(shown)-1), 0
	for i, c := range shown {
		widths[i] = boardColMin
		if m.folded(cols[c].Key) {
			widths[i] = foldedWidth
		} else {
			open++
		}
		rest -= widths[i]
	}
	for i, c := range shown { // spread the rest over the open columns
		if !m.folded(cols[c].Key) && open > 0 {
			extra := rest / open
			widths[i], rest, open = widths[i]+extra, rest-extra, open-1
		}
	}
	if m.bp == layout.XS {
		widths[0] = m.width
	}
	blocks := make([][]string, len(shown))
	for i, c := range shown {
		if m.folded(cols[c].Key) {
			blocks[i] = m.renderFolded(cols[c].Label, len(cards[c]), c == m.board.col, height)
		} else {
			blocks[i] = m.renderBoardColumn(cols, c, cards[c], widths[i], height)
		}
	}
	for row := 0; row < height; row++ {
		parts := make([]string, len(shown))
		for i := range shown {
			parts[i] = pad(blocks[i][row], widths[i])
		}
		lines = append(lines, ansi.Truncate(strings.Join(parts, " "), m.width, ""))
	}
	return append(lines, m.renderBoardStatus(cols))
}

func (m Model) columnDot(c group) string {
	if role := m.optionRole(m.boardField(), c.Key); role != "" {
		return m.theme.Fg(role).Render(m.theme.Symbol("dot")) + " "
	}
	return ""
}

// renderBoardColumn draws a column header (name, count, sums, a rule) and the cards that fit;
// the focused column scrolls to its card, the others show their top.
func (m Model) renderBoardColumn(cols []group, ci int, cards []*project.Item, w, height int) []string {
	t, c := m.theme, cols[ci]
	focused := ci == m.board.col
	count, sums := tally(cards, sumFields(m.settings.SumFor(m.currentView().Name))) // the shown cards: a lifted one counts at its target
	var meta []string
	if m.settings.CountFor(m.currentView().Name) {
		meta = append(meta, strconv.Itoa(count))
	}
	label := lipgloss.NewStyle().Bold(true).Render(c.Label)
	left := " " + m.columnDot(c) + label + " " + t.Fg("muted").Render(strings.Join(meta, ""))
	var sumText []string
	for _, s := range sums {
		sumText = append(sumText, "Σ "+s.Field+" "+strconv.FormatFloat(s.Value, 'f', -1, 64))
	}
	right := t.Fg("muted").Render(strings.Join(sumText, " · ")) + " "
	if m.bp == layout.XS { // one column: say where it sits among the others
		if ci > 0 {
			left = t.Fg("muted").Render(" "+t.Symbol("prev")) + left
		}
		right = t.Fg("muted").Render(fmt.Sprintf("%d/%d ", ci+1, len(cols)))
		if ci < len(cols)-1 {
			right = t.Fg("muted").Render(fmt.Sprintf("%d/%d %s ", ci+1, len(cols), t.Symbol("next")))
		}
	}
	head := ansi.Truncate(left, max(0, w-ansi.StringWidth(right)-1), "…")
	head += strings.Repeat(" ", max(1, w-ansi.StringWidth(head)-ansi.StringWidth(right))) + right
	rule, role := t.Symbol("box.h"), m.optionRole(m.boardField(), c.Key)
	if focused {
		rule = t.Symbol("rule.focus")
	}
	if role == "" {
		role = "border"
	}
	lines := []string{head, t.Fg(role).Render(strings.Repeat(rule, w))}

	avail := height - boardHeadLen
	from, fit := 0, len(cards)
	if len(cards)*m.cardHeight() > avail {
		fit = max(1, (avail-1)/m.cardHeight())
		if focused {
			from = max(0, min(m.board.card-fit+1, len(cards)-fit))
		}
	}
	for i := from; i < len(cards) && i < from+fit; i++ {
		lines = append(lines, m.renderBoardCard(cards[i], w, focused && i == m.board.card)...)
	}
	if more := len(cards) - from - fit; more > 0 {
		lines = append(lines, t.Fg("muted").Render(fmt.Sprintf(" ↓ %d more", more)))
	}
	if len(cards) == 0 {
		lines = append(lines, t.Fg("muted").Render(" No items"))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines[:height]
}

// renderFolded is a folded column: a narrow strip with its name and count written downwards.
func (m Model) renderFolded(label string, count int, focused bool, height int) []string {
	t := m.theme
	st := t.Fg("muted")
	if focused {
		st = t.Fg("accent").Bold(true)
	}
	lines := []string{" " + st.Render(t.Symbol("collapsed"))}
	for _, r := range []rune(label + " " + strconv.Itoa(count)) {
		lines = append(lines, " "+st.Render(string(r)))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines[:height]
}

// cardFields are the view's visible fields shown on cards: all but the title and the column field.
func (m Model) cardFields() []string {
	var out []string
	for _, f := range m.currentView().VisibleFields {
		if f != "Title" && f != m.boardField() {
			out = append(out, f)
		}
	}
	return out
}

// cardHeight: border, ref line, title, a line for the other fields when the view shows more than one, border.
func (m Model) cardHeight() int {
	if len(m.cardFields()) > 1 {
		return 5
	}
	return 4
}

// renderBoardCard is a boxed card: `symbol repo#n` with the first field value on the right, the title,
// then the other field values. The focused card has an accent border; a lifted one says MOVING.
func (m Model) renderBoardCard(it *project.Item, w int, focused bool) []string {
	t := m.theme
	inner := max(1, w-4)
	var vals []string
	for _, f := range m.cardFields() {
		if it.Values[f].Text != "" {
			_, styled := m.cell(it, f)
			vals = append(vals, styled)
		}
	}
	right := ""
	if len(vals) > 0 {
		right, vals = vals[0], vals[1:]
	}
	if lf := m.board.lifted; lf != nil && lf.item == it {
		right = t.Fg("accent").Bold(true).Render("MOVING")
	}
	first := m.symbol(it) + " " + t.Fg("muted").Render(prefix(it))
	first = ansi.Truncate(first, max(0, inner-ansi.StringWidth(right)-1), "…")
	first += strings.Repeat(" ", max(1, inner-ansi.StringWidth(first)-ansi.StringWidth(right))) + right
	body := []string{first, it.Title}
	if m.cardHeight() == 5 {
		body = append(body, t.Fg("muted").Render(strings.Join(vals, t.Fg("muted").Render(" · "))))
	}

	border := t.Fg("border")
	if focused {
		border = t.Fg("accent")
	}
	h, v := strings.Repeat(t.Symbol("box.h"), max(0, w-2)), border.Render(t.Symbol("box.v"))
	lines := []string{border.Render(t.Symbol("box.tl") + h + t.Symbol("box.tr"))}
	for _, b := range body {
		lines = append(lines, v+" "+pad(b, inner)+" "+v)
	}
	return append(lines, border.Render(t.Symbol("box.bl")+h+t.Symbol("box.br")))
}

func (m Model) renderBoardStatus(cols []group) string {
	t, k, b := m.theme, m.keys, m.board
	mode := "BOARD"
	if m.bp == layout.XS {
		mode = "BRD"
	}
	left := t.Badge("accent").Render(" " + mode + " ")
	pairs := [][2]string{
		{k.Left.Help().Key + "/" + k.Right.Help().Key, "column"},
		{k.Down.Help().Key + "/" + k.Up.Help().Key, "card"},
		{k.Move.Help().Key, "move"},
		{k.Edit.Help().Key, "edit"},
		{k.Toggle.Help().Key, "fold column"},
		{k.NextView.Help().Key, "view"},
		{k.Switch.Help().Key, "projects"},
		{k.Quit.Help().Key, "quit"},
	}
	switch {
	case b.lifted != nil:
		left += t.Badge("warning").Render(" MOVE " + cardRef(b.lifted.item) + " ")
		pairs = [][2]string{
			{k.Left.Help().Key + "/" + k.Right.Help().Key, "column"},
			{k.Down.Help().Key + "/" + k.Up.Help().Key, "position"},
			{k.Toggle.Help().Key, "drop"},
			{k.Cancel.Help().Key, "cancel"},
		}
	case m.edit != nil:
		left += t.Badge("muted").Render(" EDIT · " + m.editTitle() + " ")
		pairs = [][2]string{{"enter", "save"}, {"esc", "cancel"}}
	}
	right := ""
	if len(cols) > 0 {
		n := len(m.cards(cols)[b.col])
		at := strconv.Itoa(b.card + 1)
		if n == 0 || m.folded(cols[b.col].Key) {
			at = "–"
		}
		right = fmt.Sprintf("col %d/%d · card %s/%d ", b.col+1, len(cols), at, n)
		if m.bp == layout.XS {
			right = fmt.Sprintf("%s/%d ", at, n)
		}
	}
	right = t.Fg("muted").Render(right)
	room := m.width - ansi.StringWidth(left) - ansi.StringWidth(right) - 3
	var hints string
	if m.status != "" {
		st := t.Fg("muted")
		if m.statusErr {
			st = t.Fg("danger")
		}
		hints = st.Render(m.status)
	} else {
		for _, p := range pairs {
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
	return m.spread(left+"  "+ansi.Truncate(hints, max(0, room), "…"), right)
}
