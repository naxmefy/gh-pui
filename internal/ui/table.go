package ui

import (
	"sort"
	"strings"

	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/layout"
)

type group struct {
	Key   string // the group field's value; "" for items without one
	Label string
	Items []*project.Item
	Sums  []sum
}

type sum struct {
	Field string
	Value float64
}

// row is one line of the table body: a group header (item == nil) or an item.
type row struct {
	group int
	item  *project.Item
	depth int // nesting below a parent issue (show hierarchy)
}

// viewItems returns the items the view shows: not archived, matching the view's saved filter, sorted.
func viewItems(p *project.Project, v project.View) []*project.Item {
	var match map[string]bool
	if v.ItemIDs != nil {
		match = make(map[string]bool, len(v.ItemIDs))
		for _, id := range v.ItemIDs {
			match[id] = true
		}
	}
	var items []*project.Item
	for i := range p.Items {
		it := &p.Items[i]
		if it.Archived || (match != nil && !match[it.ID]) {
			continue
		}
		items = append(items, it)
	}
	if len(v.SortBy) > 0 {
		s := v.SortBy[0]
		sort.SliceStable(items, func(i, j int) bool {
			a, b := items[i].Values[s.Field], items[j].Values[s.Field]
			less := a.Text < b.Text
			if a.Number != nil && b.Number != nil {
				less = *a.Number < *b.Number
			}
			if s.Direction == "DESC" {
				return !less && a.Text != b.Text
			}
			return less
		})
	}
	return items
}

// estimationNames are the number fields summed when projects.yml does not say otherwise.
var estimationNames = []string{"estimate", "estimation", "story points", "points"}

// sumFields picks the NUMBER fields summed in group headers: configured ones, else the estimation field, else none.
func sumFields(p *project.Project, configured *[]string) []string {
	if configured != nil {
		return *configured
	}
	for _, f := range p.Fields {
		if f.DataType == "NUMBER" && contains(estimationNames, strings.ToLower(f.Name)) {
			return []string{f.Name}
		}
	}
	return nil
}

// sliceValues are the values of the slice field among items, in option order, then first appearance, empty last.
func sliceValues(p *project.Project, field string, items []*project.Item) []string {
	var out []string
	for _, g := range groupItems(p, project.View{GroupBy: []string{field}}, items, nil) {
		out = append(out, g.Key)
	}
	return out
}

// sliceItems keeps the items whose slice field has value.
func sliceItems(items []*project.Item, field, value string) []*project.Item {
	var out []*project.Item
	for _, it := range items {
		if it.Values[field].Text == value {
			out = append(out, it)
		}
	}
	return out
}

// groupItems groups by the view's group field in the field's own option/iteration order, empty values last.
func groupItems(p *project.Project, v project.View, items []*project.Item, sum []string) []group {
	field := ""
	if len(v.GroupBy) > 0 {
		field = v.GroupBy[0]
	}
	var order []string
	for _, f := range p.Fields {
		if f.Name != field {
			continue
		}
		for _, o := range f.Options {
			order = append(order, o.Name)
		}
		for _, it := range f.Iterations {
			order = append(order, it.Title)
		}
	}

	byKey := map[string][]*project.Item{}
	for _, it := range items {
		k := ""
		if field != "" {
			k = it.Values[field].Text
		}
		if _, seen := byKey[k]; !seen && k != "" && !contains(order, k) {
			order = append(order, k)
		}
		byKey[k] = append(byKey[k], it)
	}
	order = append(order, "")

	var groups []group
	for _, k := range order {
		its, ok := byKey[k]
		if !ok {
			continue
		}
		label := k
		if k == "" && field != "" {
			label = "No " + field
		}
		groups = append(groups, group{Key: k, Label: label, Items: its, Sums: sums(its, sum)})
	}
	return groups
}

func sums(items []*project.Item, fields []string) []sum {
	var out []sum
	for _, f := range fields {
		total, any := 0.0, false
		for _, it := range items {
			if n := it.Values[f].Number; n != nil {
				total, any = total+*n, true
			}
		}
		if any {
			out = append(out, sum{Field: f, Value: total})
		}
	}
	return out
}

// nest orders items so sub-issues follow their parent (depth+1) when the parent is among them;
// the rest keep their order at depth 0.
func nest(items []*project.Item) ([]*project.Item, []int) {
	present := map[string]bool{}
	for _, it := range items {
		if r := it.Ref(); r != "" {
			present[r] = true
		}
	}
	children := map[string][]*project.Item{}
	var roots []*project.Item
	for _, it := range items {
		if it.Parent != "" && present[it.Parent] && it.Parent != it.Ref() {
			children[it.Parent] = append(children[it.Parent], it)
		} else {
			roots = append(roots, it)
		}
	}
	var out []*project.Item
	var depths []int
	seen := map[*project.Item]bool{}
	var walk func(it *project.Item, d int)
	walk = func(it *project.Item, d int) {
		if seen[it] {
			return
		}
		seen[it] = true
		out, depths = append(out, it), append(depths, d)
		for _, c := range children[it.Ref()] {
			walk(c, d+1)
		}
	}
	for _, it := range roots {
		walk(it, 0)
	}
	for _, it := range items { // parent cycles: nothing is lost
		walk(it, 0)
	}
	return out, depths
}

// rows flattens groups into table lines; an ungrouped view has no header lines.
func rows(groups []group, grouped bool, collapsed map[string]bool, hierarchy bool) []row {
	var out []row
	for gi, g := range groups {
		if grouped {
			out = append(out, row{group: gi})
			if collapsed[g.Key] {
				continue
			}
		}
		items, depths := g.Items, make([]int, len(g.Items))
		if hierarchy {
			items, depths = nest(g.Items)
		}
		for i, it := range items {
			out = append(out, row{group: gi, item: it, depth: depths[i]})
		}
	}
	return out
}

// columns picks the view's visible fields in their order, Title first, as many as the breakpoint allows.
// Like github.com, the group field stays a column when it is visible.
func columns(v project.View, bp layout.Breakpoint) []string {
	cols := []string{"Title"}
	for _, f := range v.VisibleFields {
		if f != "Title" {
			cols = append(cols, f)
		}
	}
	limit := map[layout.Breakpoint]int{layout.XS: 1, layout.SM: 2, layout.MD: 4, layout.LG: 6, layout.XL: len(cols)}[bp]
	if len(cols) > limit {
		cols = cols[:limit]
	}
	return cols
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
