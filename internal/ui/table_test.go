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
	groups := groupItems(p, v, viewItems(p, v), sumFields(p, nil))

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
	if groups[2].Sums != nil {
		t.Errorf("group without numbers should have no sums, got %v", groups[2].Sums)
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
	groups := groupItems(p, v, viewItems(p, v), sumFields(p, nil))
	if n := len(rows(groups, true, nil, false)); n != 3+4 {
		t.Errorf("expanded rows = %d, want 7", n)
	}
	if n := len(rows(groups, true, map[string]bool{"Todo": true}, false)); n != 3+2 {
		t.Errorf("rows with Todo collapsed = %d, want 5", n)
	}
	if n := len(rows(groups, false, nil, false)); n != 4 {
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

func TestNestPutsSubIssuesUnderParents(t *testing.T) {
	it := func(n int, parent string) *project.Item {
		return &project.Item{Title: string(rune('A' + n - 1)), Repo: "r/x", Number: n, Parent: parent}
	}
	// C is a child of A, D a child of C, E's parent is not in the list, F and G point at each other
	items := []*project.Item{it(3, "r/x#1"), it(1, ""), it(2, ""), it(4, "r/x#3"), it(5, "r/x#99"), it(6, "r/x#7"), it(7, "r/x#6")}
	got, depths := nest(items)
	if names := titles(got); !reflect.DeepEqual(names, []string{"A", "C", "D", "B", "E", "F", "G"}) {
		t.Errorf("order %v", names)
	}
	if !reflect.DeepEqual(depths, []int{0, 1, 2, 0, 0, 0, 1}) {
		t.Errorf("depths %v", depths)
	}
}

func TestSumFieldsDefaultAndConfigured(t *testing.T) {
	p := testProject()
	p.Fields = append(p.Fields, project.Field{Name: "Hours", DataType: "NUMBER"})
	if got := sumFields(p, nil); !reflect.DeepEqual(got, []string{"Estimate"}) {
		t.Errorf("default: %v (only the estimation field, not Hours)", got)
	}
	none, hours := []string{}, []string{"Hours"}
	if got := sumFields(p, &none); len(got) != 0 {
		t.Errorf("sum: [] should count only, got %v", got)
	}
	if got := sumFields(p, &hours); !reflect.DeepEqual(got, hours) {
		t.Errorf("configured: %v", got)
	}
	p.Fields = p.Fields[:1] // no number fields at all
	if got := sumFields(p, nil); got != nil {
		t.Errorf("without an estimation field: %v", got)
	}
}
