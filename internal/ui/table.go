package ui

import (
	"sort"

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

// groupItems groups by the view's group field in the field's own option/iteration order, empty values last.
func groupItems(p *project.Project, v project.View, items []*project.Item) []group {
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
		groups = append(groups, group{Key: k, Label: label, Items: its, Sums: sums(p, its)})
	}
	return groups
}

func sums(p *project.Project, items []*project.Item) []sum {
	var out []sum
	for _, f := range p.Fields {
		if f.DataType != "NUMBER" {
			continue
		}
		total, any := 0.0, false
		for _, it := range items {
			if n := it.Values[f.Name].Number; n != nil {
				total, any = total+*n, true
			}
		}
		if any {
			out = append(out, sum{Field: f.Name, Value: total})
		}
	}
	return out
}

// rows flattens groups into table lines; an ungrouped view has no header lines.
func rows(groups []group, grouped bool, collapsed map[string]bool) []row {
	var out []row
	for gi, g := range groups {
		if grouped {
			out = append(out, row{group: gi})
			if collapsed[g.Key] {
				continue
			}
		}
		for _, it := range g.Items {
			out = append(out, row{group: gi, item: it})
		}
	}
	return out
}

// columns picks the view's visible fields, Title first and the group field left out, as many as the breakpoint allows.
func columns(v project.View, bp layout.Breakpoint) []string {
	cols := []string{"Title"}
	for _, f := range v.VisibleFields {
		if f != "Title" && (len(v.GroupBy) == 0 || f != v.GroupBy[0]) {
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
