package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/naxmefy/gh-pui/internal/gh"
	"github.com/naxmefy/gh-pui/internal/project"
)

type saveCall struct {
	item, field string
	v           gh.FieldValue
}

// newEditModel is newTestModel with ids and a save func that records calls and fails with err.
func newEditModel(t *testing.T, err error) (tea.Model, *[]saveCall) {
	t.Helper()
	m := newTestModel(t).(Model)
	p := m.project
	p.ID = "P"
	p.Fields[0].ID, p.Fields[1].ID = "F-status", "F-est"
	for i := range p.Fields[0].Options {
		p.Fields[0].Options[i].ID = "O-" + p.Fields[0].Options[i].Name
	}
	var calls []saveCall
	m = m.WithSave(func(pid, item, field string, v gh.FieldValue) error {
		if pid != "P" {
			t.Errorf("project id %q", pid)
		}
		calls = append(calls, saveCall{item, field, v})
		return err
	})
	return m, &calls
}

func TestEditStatusThroughFieldChooser(t *testing.T) {
	m, calls := newEditModel(t, nil)
	m = drive(m, "j", "e") // cursor on B, title column → choose the field
	if s := screen(m); !strings.Contains(s, "Edit field") || !strings.Contains(s, "Status") {
		t.Fatalf("chooser not shown:\n%s", s)
	}
	m = drive(m, "enter", "down", "enter") // Status → from Todo down to In Progress
	if len(*calls) != 1 || (*calls)[0].item != "b" || (*calls)[0].field != "F-status" || (*calls)[0].v.OptionID != "O-In Progress" {
		t.Fatalf("calls %+v", *calls)
	}
	s := screen(m)
	if !strings.Contains(s, "In Progress  1 item · Σ Estimate 5") || !strings.Contains(s, "Todo  1 item · Σ Estimate 1") {
		t.Errorf("B not regrouped:\n%s", s)
	}
	if strings.Contains(s, "saving") {
		t.Errorf("status should clear after a successful save:\n%s", s)
	}
}

func TestEditNumberInColumnUpdatesSums(t *testing.T) {
	m, calls := newEditModel(t, nil)
	m = drive(m, "j", "l", "l", "e", "backspace", "8", "enter")
	if len(*calls) != 1 || *(*calls)[0].v.Number != 8 {
		t.Fatalf("calls %+v", *calls)
	}
	if s := screen(m); !strings.Contains(s, "Todo  2 items · Σ Estimate 9") {
		t.Errorf("sum not updated:\n%s", s)
	}
}

func TestEditRejectsBadNumberAndCancels(t *testing.T) {
	m, calls := newEditModel(t, nil)
	m = drive(m, "j", "l", "l", "e", "x", "enter")
	if s := screen(m); !strings.Contains(s, `"5x" is not a number`) {
		t.Fatalf("validation error missing:\n%s", s)
	}
	m = drive(m, "esc")
	if s := screen(m); strings.Contains(s, "enter save") || len(*calls) != 0 || !strings.Contains(s, "Todo  2 items · Σ Estimate 6") {
		t.Errorf("esc should close without saving:\n%s", s)
	}
}

func TestSaveErrorRollsBack(t *testing.T) {
	m, _ := newEditModel(t, errors.New("forbidden"))
	m = drive(m, "j", "l", "l", "e", "backspace", "9", "enter")
	s := screen(m)
	if !strings.Contains(s, "Estimate not saved: forbidden") || !strings.Contains(s, "Todo  2 items · Σ Estimate 6") {
		t.Errorf("no rollback:\n%s", s)
	}
}

func TestEditorResults(t *testing.T) {
	it := &project.Item{Title: "x", Values: map[string]project.Value{"Tags": {Text: "a, c"}, "Sprint": {Text: "S2"}}}
	multi := project.Field{Name: "Tags", DataType: "MULTI_SELECT", Options: []project.Option{{ID: "1", Name: "a"}, {ID: "2", Name: "b"}, {ID: "3", Name: "c"}}}
	e := newEditor(it, multi)
	e.update(keyMsg("down"))
	e.update(keyMsg("space")) // add b
	e.update(keyMsg("down"))
	e.update(keyMsg("space")) // remove c
	v, val, _ := e.result()
	if strings.Join(v.OptionIDs, ",") != "1,2" || val.Text != "a, b" {
		t.Errorf("multi: %v %q", v.OptionIDs, val.Text)
	}

	iter := project.Field{Name: "Sprint", DataType: "ITERATION", Iterations: []project.Iteration{{ID: "i1", Title: "S1"}, {ID: "i2", Title: "S2"}}}
	e = newEditor(it, iter)
	if v, val, _ := e.result(); v.IterationID != "i2" || val.Text != "S2" {
		t.Errorf("iteration should start on the current value: %+v %q", v, val.Text)
	}
	e.update(keyMsg("down")) // the clear row
	if v, _, _ := e.result(); v.IterationID != "" || v.OptionID != "" {
		t.Errorf("clear row should clear, got %+v", v)
	}

	date := project.Field{Name: "Due", DataType: "DATE"}
	e = newEditor(it, date)
	for _, k := range []string{"2", "0", "2", "6", "-", "1", "3", "-", "4", "0"} {
		e.update(keyMsg(k))
	}
	if _, _, err := e.result(); err == nil {
		t.Error("2026-13-40 should be rejected")
	}
}
