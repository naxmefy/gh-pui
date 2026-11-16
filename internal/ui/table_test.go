package ui

import (
	"reflect"
	"testing"

	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/layout"
)

func num(f float64) *float64 { return &f }

func testProject() *project.Project {
	return &project.Project{
		Fields: []project.Field{
			{Name: "Status", DataType: "SINGLE_SELECT", Options: []project.Option{{Name: "Todo"}, {Name: "In Progress"}, {Name: "Done"}}},
			{Name: "Estimate", DataType: "NUMBER"},
		},
		Items: []project.Item{
			{ID: "a", Title: "A", Values: map[string]project.Value{"Status": {Text: "Done"}, "Estimate": {Text: "2", Number: num(2)}}},
			{ID: "b", Title: "B", Values: map[string]project.Value{"Status": {Text: "Todo"}, "Estimate": {Text: "5", Number: num(5)}}},
			{ID: "c", Title: "C", Values: map[string]project.Value{}},
			{ID: "d", Title: "D", Values: map[string]project.Value{"Status": {Text: "Todo"}, "Estimate": {Text: "1", Number: num(1)}}},
			{ID: "e", Title: "E", Archived: true, Values: map[string]project.Value{"Status": {Text: "Todo"}}},
		},
	}
}

func titles(items []*project.Item) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.Title)
	}
	return out
}

func TestGroupingOrderAndSums(t *testing.T) {
	p := testProject()
	v := project.View{GroupBy: []string{"Status"}}
	groups := groupItems(p, v, viewItems(p, v), []string{"Estimate"})

	var labels []string
	for _, g := range groups {
		labels = append(labels, g.Label)
	}
	if want := []string{"Todo", "Done", "No Status"}; !reflect.DeepEqual(labels, want) {
		t.Fatalf("groups = %v, want %v (option order, empty last, archived dropped)", labels, want)
	}
	if got := titles(groups[0].Items); !reflect.DeepEqual(got, []string{"B", "D"}) {
		t.Errorf("todo items = %v", got)
	}
	if want := []sum{{"Estimate", 6}}; !reflect.DeepEqual(groups[0].Sums, want) {
		t.Errorf("todo sums = %v", groups[0].Sums)
	}
	if want := []sum{{"Estimate", 0}}; !reflect.DeepEqual(groups[2].Sums, want) {
		t.Errorf("group without numbers should sum to 0, got %v", groups[2].Sums)
	}
}

func TestFilterAndSort(t *testing.T) {
	p := testProject()
	v := project.View{ItemIDs: []string{"a", "b", "d"}, SortBy: []project.Sort{{Field: "Estimate", Direction: "DESC"}}}
	if got := titles(viewItems(p, v)); !reflect.DeepEqual(got, []string{"B", "A", "D"}) {
		t.Errorf("filtered + sorted = %v", got)
	}
	v.SortBy[0].Direction = "ASC"
	if got := titles(viewItems(p, v)); !reflect.DeepEqual(got, []string{"D", "A", "B"}) {
		t.Errorf("ascending = %v", got)
	}
}

func TestRowsCollapse(t *testing.T) {
	p := testProject()
	v := project.View{GroupBy: []string{"Status"}}
	groups := groupItems(p, v, viewItems(p, v), []string{"Estimate"})
	if n := len(rows(groups, true, nil, nil, nil)); n != 3+4 {
		t.Errorf("expanded rows = %d, want 7", n)
	}
	if n := len(rows(groups, true, map[string]bool{"Todo": true}, nil, nil)); n != 3+2 {
		t.Errorf("rows with Todo collapsed = %d, want 5", n)
	}
	if n := len(rows(groups, false, nil, nil, nil)); n != 4 {
		t.Errorf("ungrouped rows = %d, want 4", n)
	}
}

func TestColumnsPerBreakpoint(t *testing.T) {
	v := project.View{GroupBy: []string{"Status"}, VisibleFields: []string{"Title", "Assignees", "Status", "Labels", "Estimate", "Milestone", "Repository", "Linked pull requests"}}
	for bp, want := range map[layout.Breakpoint]int{layout.XS: 1, layout.SM: 2, layout.MD: 4, layout.LG: 6, layout.XL: 8} {
		cols := columns(v, bp)
		if len(cols) != want || cols[0] != "Title" || (want >= 3 && cols[2] != "Status") {
			t.Errorf("%s: %v", bp, cols)
		}
	}
}

func TestRootsAndNestFollowGitHubHierarchy(t *testing.T) {
	it := func(n int, parent string) *project.Item {
		return &project.Item{Title: string(rune('A' + n - 1)), Repo: "r/x", Number: n, Parent: parent}
	}
	// C is a child of A, D a child of C, E's parent is not in the list, F and G point at each other
	all := []*project.Item{it(3, "r/x#1"), it(1, ""), it(2, ""), it(4, "r/x#3"), it(5, "r/x#99"), it(6, "r/x#7"), it(7, "r/x#6")}
	rs := roots(all)
	if names := titles(rs); !reflect.DeepEqual(names, []string{"A", "B", "E", "F"}) {
		t.Errorf("roots %v (a parent cycle keeps one of its items as root)", names)
	}
	got, depths := nest(rs, all)
	if names := titles(got); !reflect.DeepEqual(names, []string{"A", "C", "D", "B", "E", "F", "G"}) {
		t.Errorf("order %v", names)
	}
	if !reflect.DeepEqual(depths, []int{0, 1, 2, 0, 0, 0, 1}) {
		t.Errorf("depths %v", depths)
	}
}

func TestTallyCountsAndSums(t *testing.T) {
	p := testProject()
	items := viewItems(p, project.View{})
	if n, s := tally(items, nil); n != 4 || s != nil {
		t.Errorf("count only: %d %v", n, s)
	}
	if n, s := tally(items, []string{"Estimate", "Missing"}); n != 4 || !reflect.DeepEqual(s, []sum{{"Estimate", 8}, {"Missing", 0}}) {
		t.Errorf("with Estimate: %d %v (a field without values sums to 0)", n, s)
	}
	if got := sumFields(nil); got != nil {
		t.Errorf("no field sum configured means count only, got %v", got)
	}
}

func TestRowsHideSubIssuesUntilExpanded(t *testing.T) {
	parent := &project.Item{Title: "P", Repo: "r/x", Number: 1}
	child := &project.Item{Title: "C", Repo: "r/x", Number: 2, Parent: "r/x#1"}
	grand := &project.Item{Title: "G", Repo: "r/x", Number: 3, Parent: "r/x#2"}
	other := &project.Item{Title: "O", Repo: "r/x", Number: 4}
	all := []*project.Item{parent, child, grand, other}
	groups := []group{{Items: roots(all)}}
	names := func(rs []row) (out []string) {
		for _, r := range rs {
			out = append(out, r.item.Title)
		}
		return out
	}
	rs := rows(groups, false, nil, all, nil)
	if got := names(rs); !reflect.DeepEqual(got, []string{"P", "O"}) || rs[0].children != 1 {
		t.Errorf("collapsed: %v children=%d", got, rs[0].children)
	}
	if got := names(rows(groups, false, nil, all, map[string]bool{"r/x#1": true})); !reflect.DeepEqual(got, []string{"P", "C", "O"}) {
		t.Errorf("parent expanded: %v", got)
	}
	if got := names(rows(groups, false, nil, all, map[string]bool{"r/x#1": true, "r/x#2": true})); !reflect.DeepEqual(got, []string{"P", "C", "G", "O"}) {
		t.Errorf("all expanded: %v", got)
	}
}
