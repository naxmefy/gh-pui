package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/naxmefy/gh-pui/internal/gh"
	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/theme"
)

// SaveFunc writes a field value to GitHub; gh.Client.UpdateField in production.
type SaveFunc func(projectID, itemID, fieldID string, v gh.FieldValue) error

// MoveFunc puts an item after afterID in the project's order ("" = top); gh.Client.MoveItem in production.
type MoveFunc func(projectID, itemID, afterID string) error

// savedMsg reports a finished write; on error the optimistic local value is rolled back to old.
type savedMsg struct {
	id    string // item id: items can be reordered (board moves) while a save is in flight
	field string
	old   project.Value
	err   error
}

func editable(f project.Field) bool {
	switch f.DataType {
	case "TEXT", "NUMBER", "DATE", "SINGLE_SELECT", "MULTI_SELECT", "ITERATION":
		return true
	}
	return false
}

type editKind int

const (
	chooseField editKind = iota // pick which field to edit
	pickOne                     // single select, iteration
	pickMany                    // multi select
	typeIn                      // text, number, date
)

const clearLabel = "✕ clear"

// editor is the popup for one item: a field chooser, an option list or a text input.
type editor struct {
	kind   editKind
	item   *project.Item
	field  project.Field
	fields []project.Field // chooseField
	labels []string        // rows of the list kinds
	ids    []string        // option or iteration id per label; "" = clear
	chosen map[int]bool    // pickMany
	cursor int
	input  textinput.Model
	err    string
}

func newChooser(p *project.Project, it *project.Item) *editor {
	e := &editor{kind: chooseField, item: it}
	for _, f := range p.Fields {
		if editable(f) {
			e.fields = append(e.fields, f)
			e.labels = append(e.labels, f.Name)
		}
	}
	return e
}

func newEditor(it *project.Item, f project.Field) *editor {
	e := &editor{item: it, field: f}
	cur := it.Values[f.Name].Text
	switch f.DataType {
	case "SINGLE_SELECT", "MULTI_SELECT":
		e.kind, e.chosen = pickOne, map[int]bool{}
		if f.DataType == "MULTI_SELECT" {
			e.kind = pickMany
		}
		selected := strings.Split(cur, ", ")
		for i, o := range f.Options {
			e.labels, e.ids = append(e.labels, o.Name), append(e.ids, o.ID)
			if contains(selected, o.Name) {
				e.chosen[i] = true
				if e.kind == pickOne {
					e.cursor = i
				}
			}
		}
	case "ITERATION":
		e.kind = pickOne
		for i, it := range f.Iterations {
			label := it.Title + "  " + it.StartDate
			if it.Completed {
				label += "  (done)"
			}
			e.labels, e.ids = append(e.labels, label), append(e.ids, it.ID)
			if it.Title == cur {
				e.cursor = i
			}
		}
	default:
		e.kind = typeIn
		e.input = textinput.New()
		e.input.SetValue(cur)
		e.input.CursorEnd()
		if f.DataType == "DATE" {
			e.input.Placeholder = "YYYY-MM-DD"
		}
		e.input.Focus()
	}
	if e.kind == pickOne {
		e.labels, e.ids = append(e.labels, clearLabel), append(e.ids, "")
	}
	return e
}

// result turns the editor state into the API value and the value shown locally.
func (e *editor) result() (gh.FieldValue, project.Value, error) {
	switch e.kind {
	case pickOne:
		id := e.ids[e.cursor]
		if id == "" {
			return gh.FieldValue{}, project.Value{}, nil
		}
		text := e.labels[e.cursor]
		if e.field.DataType == "ITERATION" {
			text = e.field.Iterations[e.cursor].Title
			return gh.FieldValue{IterationID: id}, project.Value{Text: text}, nil
		}
		return gh.FieldValue{OptionID: id}, project.Value{Text: text}, nil
	case pickMany:
		var ids, names []string
		for i := range e.labels {
			if e.chosen[i] {
				ids, names = append(ids, e.ids[i]), append(names, e.labels[i])
			}
		}
		return gh.FieldValue{OptionIDs: ids}, project.Value{Text: strings.Join(names, ", ")}, nil
	}
	s := strings.TrimSpace(e.input.Value())
	if s == "" {
		return gh.FieldValue{}, project.Value{}, nil
	}
	switch e.field.DataType {
	case "NUMBER":
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return gh.FieldValue{}, project.Value{}, fmt.Errorf("%q is not a number", s)
		}
		return gh.FieldValue{Number: &n}, project.Value{Text: strconv.FormatFloat(n, 'f', -1, 64), Number: &n}, nil
	case "DATE":
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return gh.FieldValue{}, project.Value{}, fmt.Errorf("%q is not a date (YYYY-MM-DD)", s)
		}
		return gh.FieldValue{Date: s}, project.Value{Text: s}, nil
	}
	return gh.FieldValue{Text: &s}, project.Value{Text: s}, nil
}

// update handles a key; done means the editor closes, with apply=true when the value should be saved.
func (e *editor) update(msg tea.KeyPressMsg) (cmd tea.Cmd, done, apply bool) {
	switch msg.String() {
	case "esc":
		return nil, true, false
	case "enter":
		if e.kind == chooseField {
			if len(e.fields) > 0 {
				*e = *newEditor(e.item, e.fields[e.cursor])
			}
			return nil, false, false
		}
		return nil, true, true
	}
	if e.kind == typeIn {
		e.err = ""
		e.input, cmd = e.input.Update(msg)
		return cmd, false, false
	}
	switch msg.String() {
	case "up", "k", "ctrl+k":
		e.cursor = max(0, e.cursor-1)
	case "down", "j", "ctrl+j":
		e.cursor = min(len(e.labels)-1, e.cursor+1)
	case "space":
		if e.kind == pickMany {
			e.chosen[e.cursor] = !e.chosen[e.cursor]
		}
	}
	return nil, false, false
}

func (e *editor) lines(t theme.Theme) []string {
	title := "Edit field"
	if e.kind != chooseField {
		title = e.field.Name
	}
	lines := []string{t.Fg("accent").Bold(true).Render(" " + title), t.Fg("muted").Render(" " + e.item.Title), ""}
	if e.kind == typeIn {
		lines = append(lines, " "+e.input.View())
	}
	for i, l := range e.labels {
		mark := "  "
		if i == e.cursor {
			mark = t.Fg("accent").Render("▌ ")
		}
		if e.kind == pickMany {
			box := "[ ] "
			if e.chosen[i] {
				box = "[x] "
			}
			l = box + l
		}
		lines = append(lines, " "+mark+l)
	}
	hint := " enter save · esc cancel"
	switch e.kind {
	case chooseField:
		hint = " enter choose · esc cancel"
	case pickMany:
		hint = " space toggle · enter save · esc cancel"
	case typeIn:
		hint = " enter save (empty clears) · esc cancel"
	}
	lines = append(lines, "", t.Fg("muted").Render(hint))
	if e.err != "" {
		lines = append(lines, t.Fg("danger").Render(" "+e.err))
	}
	return lines
}
