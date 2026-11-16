package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/naxmefy/gh-pui/internal/config"
	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/layout"
	"github.com/naxmefy/gh-pui/internal/ui/theme"
)

func newTestModel(t *testing.T) tea.Model {
	t.Helper()
	th, _ := theme.New(config.Default(), "none")
	keys, _ := NewKeyMap(nil)
	p := testProject()
	p.Owner, p.Number, p.Title = "x", 1, "demo"
	p.Views = []project.View{
		{Number: 1, Name: "By status", GroupBy: []string{"Status"}, VisibleFields: []string{"Title", "Status", "Estimate"}},
		{Number: 2, Name: "Todo only", ItemIDs: []string{"b", "d"}, VisibleFields: []string{"Title"}},
	}
	m, _ := tea.Model(NewModel("me", p, th, keys).WithSettings(config.ProjectSettings{Sum: &[]string{"Estimate"}})).
		Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

func press(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab}
		default:
			msg = tea.KeyPressMsg{Code: rune(k[0]), Text: k}
		}
		m, _ = m.Update(msg)
	}
	return m
}

func screen(m tea.Model) string { return ansi.Strip(m.(Model).View().Content) }

func TestWindowSizeSetsBreakpoint(t *testing.T) {
	m := newTestModel(t)
	for width, want := range map[int]layout.Breakpoint{40: layout.XS, 80: layout.SM, 150: layout.LG} {
		m, _ = m.Update(tea.WindowSizeMsg{Width: width, Height: 30})
		if got := m.(Model).bp; got != want {
			t.Errorf("width %d: bp %s, want %s", width, got, want)
		}
	}
}

func TestTableShowsGroupsSumsAndPeek(t *testing.T) {
	s := screen(newTestModel(t))
	for _, want := range []string{"Todo  2 items · Σ Estimate 6", "Done  1 item · Σ Estimate 2", "No Status  1 item", "Estimate", "B", "By status"} {
		if !strings.Contains(s, want) {
			t.Errorf("screen lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "○ E") {
		t.Errorf("archived item shown:\n%s", s)
	}

	// cursor down onto item B: the peek shows its fields
	s = screen(press(newTestModel(t), "j"))
	if !strings.Contains(s, "Status: Todo") {
		t.Errorf("peek lacks status of B:\n%s", s)
	}
}

func TestFoldGroupAndSwitchView(t *testing.T) {
	m := press(newTestModel(t), "j", "enter") // fold from an item row folds its group
	s := screen(m)
	if strings.Contains(s, "○ D") || !strings.Contains(s, "▸ Todo") {
		t.Errorf("Todo not folded:\n%s", s)
	}
	if m.(Model).cursor != 0 {
		t.Errorf("cursor should move to the folded header, got %d", m.(Model).cursor)
	}

	s = screen(press(m, "tab"))
	if strings.Contains(s, "No Status") || !strings.Contains(s, "○ B") || !strings.Contains(s, "○ D") || strings.Contains(s, "○ A") {
		t.Errorf("filtered view wrong:\n%s", s)
	}
	if s2 := screen(press(m, "2")); s2 != s {
		t.Errorf("digit shortcut should select view 2")
	}
}

func TestXSCardsScrollAndStatusbar(t *testing.T) {
	m, _ := newTestModel(t).Update(tea.WindowSizeMsg{Width: 50, Height: 8})
	m = press(m, "G") // last item C sits below several two-line cards
	s := screen(m)
	if !strings.Contains(s, "▌○") || !strings.Contains(s, "  C") || !strings.Contains(s, "TBL") || !strings.Contains(s, "4/4") {
		t.Errorf("cursor card not visible or statusbar wrong:\n%s", s)
	}
	if n := strings.Count(s, "\n") + 1; n != 8 {
		t.Errorf("screen has %d lines, want 8:\n%s", n, s)
	}
}

func TestSliceKeysNarrowTheTable(t *testing.T) {
	sliced := func() tea.Model { // fresh each time: slice state lives in a map shared by model copies
		m := newTestModel(t).(Model).WithSettings(config.ProjectSettings{Views: map[string]config.ViewSettings{"By status": {Slice: "Status"}}})
		tm, _ := tea.Model(m).Update(tea.WindowSizeMsg{Width: 120, Height: 30})
		return tm
	}
	s := screen(press(sliced(), "]")) // All → Todo
	if !strings.Contains(s, "○ B") || !strings.Contains(s, "○ D") || strings.Contains(s, "○ A") || !strings.Contains(s, "▌Todo") {
		t.Errorf("slice Todo wrong:\n%s", s)
	}
	if s := screen(press(sliced(), "[")); !strings.Contains(s, "▌No Status") || !strings.Contains(s, "○ C") || strings.Contains(s, "○ B") {
		t.Errorf("[ from All should wrap to the last value:\n%s", s)
	}
}

func TestHierarchyFoldFromSubIssueAndTopLevelSums(t *testing.T) {
	th, _ := theme.New(config.Default(), "none")
	keys, _ := NewKeyMap(nil)
	todo := project.Value{Text: "Todo"}
	p := &project.Project{
		Fields: []project.Field{{Name: "Status", DataType: "SINGLE_SELECT", Options: []project.Option{{Name: "Todo"}}}, {Name: "Estimate", DataType: "NUMBER"}},
		Views:  []project.View{{Name: "v", GroupBy: []string{"Status"}, VisibleFields: []string{"Title"}}},
		Items: []project.Item{
			{ID: "p", Title: "Parent", Repo: "r/x", Number: 1, Values: map[string]project.Value{"Status": todo, "Estimate": {Text: "5", Number: num(5)}}},
			{ID: "c", Title: "Child", Repo: "r/x", Number: 2, Parent: "r/x#1", Values: map[string]project.Value{"Status": todo, "Estimate": {Text: "3", Number: num(3)}}},
		},
	}
	m, _ := NewModel("me", p, th, keys).WithSettings(config.ProjectSettings{Sum: &[]string{"Estimate"}}).
		Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if s := screen(m); !strings.Contains(s, "Todo  1 item · Σ Estimate 5") || strings.Contains(s, "Child") {
		t.Fatalf("sub-issue should be hidden and left out of count and sum:\n%s", s)
	}
	m = press(m, "j", "enter", "j") // expand the parent, move onto the child
	if s := screen(m); !strings.Contains(s, "Child") {
		t.Fatalf("parent not expanded:\n%s", s)
	}
	m = press(m, "enter") // on the child: collapse the parent, cursor jumps to it
	if s := screen(m); strings.Contains(s, "Child") || !strings.Contains(s, "▾ Todo") {
		t.Errorf("enter on a sub-issue should collapse its parent, not the group:\n%s", s)
	}
	if c := m.(Model).cursor; c != 1 {
		t.Errorf("cursor = %d, want 1 (the parent)", c)
	}
}

// TestSubIssueCountsUnderItsParentsGroup mirrors naxmefy/18 on github.com: a Done sub-issue of a Todo parent
// shows under the parent and is neither counted nor summed in Done; a group of only such items disappears.
func TestSubIssueCountsUnderItsParentsGroup(t *testing.T) {
	th, _ := theme.New(config.Default(), "none")
	keys, _ := NewKeyMap(nil)
	val := func(status string, est float64) map[string]project.Value {
		return map[string]project.Value{"Status": {Text: status}, "Estimate": {Text: "x", Number: num(est)}}
	}
	p := &project.Project{
		Fields: []project.Field{{Name: "Status", DataType: "SINGLE_SELECT", Options: []project.Option{{Name: "Todo"}, {Name: "Done"}}}, {Name: "Estimate", DataType: "NUMBER"}},
		Views:  []project.View{{Name: "v", GroupBy: []string{"Status"}, VisibleFields: []string{"Title"}}},
		Items: []project.Item{
			{ID: "e", Title: "Epic", Repo: "r/x", Number: 1, Values: map[string]project.Value{"Status": {Text: "Todo"}}},
			{ID: "t", Title: "Task", Repo: "r/x", Number: 2, Values: val("Todo", 8)},
			{ID: "d", Title: "DoneSub", Repo: "r/x", Number: 3, Parent: "r/x#1", Values: val("Done", 5)},
		},
	}
	m, _ := NewModel("me", p, th, keys).WithSettings(config.ProjectSettings{Sum: &[]string{"Estimate"}}).
		Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	s := screen(m)
	if !strings.Contains(s, "Todo  2 items · Σ Estimate 8") || strings.Contains(s, "Done") {
		t.Errorf("want Todo 2 items Σ 8 and no Done group:\n%s", s)
	}
	m = press(m, "j", "enter")
	if s := screen(m); !strings.Contains(s, "└ x#3 DoneSub") {
		t.Errorf("the Done sub-issue should show under its Todo parent:\n%s", s)
	}
}
