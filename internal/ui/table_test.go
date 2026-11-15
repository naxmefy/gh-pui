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
	groups := groupItems(p, v, viewItems(p, v))

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
	groups := groupItems(p, v, viewItems(p, v))
	if n := len(rows(groups, true, nil)); n != 3+4 {
		t.Errorf("expanded rows = %d, want 7", n)
	}
	if n := len(rows(groups, true, map[string]bool{"Todo": true})); n != 3+2 {
		t.Errorf("rows with Todo collapsed = %d, want 5", n)
	}
	if n := len(rows(groups, false, nil)); n != 4 {
		t.Errorf("ungrouped rows = %d, want 4", n)
	}
}

func TestColumnsPerBreakpoint(t *testing.T) {
	v := project.View{GroupBy: []string{"Status"}, VisibleFields: []string{"Title", "Assignees", "Status", "Labels", "Estimate", "Milestone", "Repository", "Linked pull requests"}}
	for bp, want := range map[layout.Breakpoint]int{layout.XS: 1, layout.SM: 2, layout.MD: 4, layout.LG: 6, layout.XL: 7} {
		cols := columns(v, bp)
		if len(cols) != want || cols[0] != "Title" || contains(cols, "Status") {
			t.Errorf("%s: %v", bp, cols)
		}
	}
}
