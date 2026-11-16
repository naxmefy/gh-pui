package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/naxmefy/gh-pui/internal/gh"
	"github.com/naxmefy/gh-pui/internal/project"
)

// newBoardModel shows testProject as a board (columns: No Status [C], Todo [B, D], In Progress [], Done [A])
// and records the writes as "save item option" and "move item after".
func newBoardModel(t *testing.T, saveErr, moveErr error) (tea.Model, *[]string) {
	t.Helper()
	m := newTestModel(t).(Model)
	p := m.project
	p.ID, p.Fields[0].ID = "P", "F-status"
	for i := range p.Fields[0].Options {
		p.Fields[0].Options[i].ID = "O-" + p.Fields[0].Options[i].Name
	}
	p.Views = []project.View{{Number: 1, Name: "Board", Layout: "BOARD_LAYOUT", VisibleFields: []string{"Title", "Estimate"}}}
	var calls []string
	m = m.WithSave(func(_, item, field string, v gh.FieldValue) error {
		calls = append(calls, "save "+item+" "+v.OptionID)
		return saveErr
	}).WithMove(func(_, item, after string) error {
		calls = append(calls, "move "+item+" after "+after)
		return moveErr
	})
	return m, &calls
}

func order(m tea.Model) string {
	var ids []string
	for _, it := range m.(Model).project.Items {
		ids = append(ids, it.ID)
	}
	return strings.Join(ids, "")
}

func TestBoardColumnsIncludeEmptyOptions(t *testing.T) {
	m, _ := newBoardModel(t, nil, nil)
	var got []string
	for _, c := range m.(Model).boardColumns() {
		got = append(got, c.Label+":"+strings.Join(titles(c.Items), ""))
	}
	if s := strings.Join(got, " "); s != "No Status:C Todo:BD In Progress: Done:A" {
		t.Errorf("columns %s", s)
	}
}

func TestBoardMoveToAnotherColumn(t *testing.T) {
	m, calls := newBoardModel(t, nil, nil)
	m = drive(m, "l", "m", "l", "enter") // B from Todo to the empty In Progress
	if s := strings.Join(*calls, "; "); s != "save b O-In Progress; move b after " {
		t.Errorf("calls %s", s)
	}
	if st := m.(Model).project.Items[0].Values["Status"].Text; st != "In Progress" || order(m) != "bacde" {
		t.Errorf("status %q, order %s", st, order(m))
	}
	if s := screen(m); strings.Contains(s, "MOVE") || strings.Contains(s, "moving") {
		t.Errorf("move mode should end after the drop:\n%s", s)
	}
}

func TestBoardReorderInColumn(t *testing.T) {
	m, calls := newBoardModel(t, nil, nil)
	m = drive(m, "l", "m", "j", "enter") // B below D
	if s := strings.Join(*calls, "; "); s != "move b after d" || order(m) != "acdbe" {
		t.Errorf("calls %s, order %s", s, order(m))
	}
}

func TestBoardDropRollsBackOnError(t *testing.T) {
	m, _ := newBoardModel(t, errors.New("boom"), nil)
	m = drive(m, "l", "m", "l", "enter")
	b := m.(Model).item("b")
	if b.Values["Status"].Text != "Todo" || order(m) != "abcde" || !strings.Contains(screen(m), "move not saved: boom") {
		t.Errorf("status %q, order %s:\n%s", b.Values["Status"].Text, order(m), screen(m))
	}

	m, _ = newBoardModel(t, nil, errors.New("late"))
	m = drive(m, "l", "m", "l", "enter") // the column is saved, only the position fails
	if m.(Model).item("b").Values["Status"].Text != "In Progress" || order(m) != "abcde" {
		t.Errorf("after a failed position: order %s", order(m))
	}
}

func TestBoardCancelAndFold(t *testing.T) {
	m, calls := newBoardModel(t, nil, nil)
	m = drive(m, "l", "m", "l", "esc")
	if len(*calls) != 0 || m.(Model).board.col != 1 {
		t.Errorf("esc should put the card back: calls %v, col %d", *calls, m.(Model).board.col)
	}
	m = drive(m, "l", "enter", "h", "m", "l", "enter") // fold In Progress: the drop skips it and lands in Done
	if s := strings.Join(*calls, "; "); !strings.HasPrefix(s, "save b O-Done") {
		t.Errorf("calls %s", s)
	}
}

func TestPlace(t *testing.T) {
	items := []project.Item{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	place(items, "c", "")
	place(items, "a", "b")
	if got := items[0].ID + items[1].ID + items[2].ID; got != "cba" {
		t.Errorf("order %s", got)
	}
}
