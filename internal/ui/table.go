package ui

import (
	"sort"

	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/layout"
)

type group struct {
	Key   string          // the group field's value; "" for items without one
	Label string
	Items []*project.Item // with show hierarchy only top-level items; sub-issues hang below their parent
	Count int
	Sums  []sum
}

type sum struct {
	Field string
	Value float64
}

// row is one line of the table body: a group header (item == nil) or an item.
type row struct {
	group    int
	item     *project.Item
	depth    int // nesting below a parent issue (show hierarchy)
	children int // direct sub-issues among the group's items
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

// sumFields picks the NUMBER fields summed in group headers: only what projects.yml configures
// (github.com's field sum is a view setting the API does not expose; its default is the count alone).
func sumFields(configured *[]string) []string {
	if configured == nil {
		return nil
	}
	return *configured
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
		count, sums := tally(its, sum)
		groups = append(groups, group{Key: k, Label: label, Items: its, Count: count, Sums: sums})
	}
	return groups
}

// tally is the one place that counts: it gives the item count and the sums of fields for a list.
// Which items count is decided before, once: with show hierarchy only top-level ones (see roots),
// so the count and every sum follow the same rule, like github.com.
func tally(items []*project.Item, fields []string) (int, []sum) {
	var out []sum
	for _, f := range fields { // a configured field always shows, as 0 without values (github.com does the same)
		total := 0.0
		for _, it := range items {
			if n := it.Values[f].Number; n != nil {
				total += *n
			}
		}
		out = append(out, sum{Field: f, Value: total})
	}
	return len(items), out
}

// roots are the items that are not a sub-issue of another item in the list. A sub-issue whose parent is
// in the list belongs under that parent, wherever the parent is grouped (github.com's show hierarchy);
// items caught in a parent cycle count as roots so nothing disappears.
func roots(all []*project.Item) []*project.Item {
	present := map[string]bool{}
	for _, it := range all {
		if r := it.Ref(); r != "" {
			present[r] = true
		}
	}
	var out []*project.Item
	for _, it := range all {
		if it.Parent == "" || !present[it.Parent] || it.Parent == it.Ref() {
			out = append(out, it)
		}
	}
	nested, _ := nest(out, all)
	reached := map[*project.Item]bool{}
	for _, it := range nested {
		reached[it] = true
	}
	for _, it := range all {
		if !reached[it] {
			out = append(out, it)
			nested, _ = nest([]*project.Item{it}, all)
			for _, n := range nested {
				reached[n] = true
			}
		}
	}
	return out
}

// nest lists each root followed by its sub-issues (looked up among all), depth-first, with their depth.
func nest(rootItems, all []*project.Item) ([]*project.Item, []int) {
	children := map[string][]*project.Item{}
	for _, it := range all {
		if it.Parent != "" && it.Parent != it.Ref() {
			children[it.Parent] = append(children[it.Parent], it)
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
	for _, it := range rootItems {
		walk(it, 0)
	}
	return out, depths
}

// rows flattens groups into table lines; an ungrouped view has no header lines. With show hierarchy (all set:
// every item of the view), sub-issues follow their parent and stay hidden until it is expanded, like github.com.
func rows(groups []group, grouped bool, collapsed map[string]bool, all []*project.Item, expanded map[string]bool) []row {
	var out []row
	for gi, g := range groups {
		if grouped {
			out = append(out, row{group: gi})
			if collapsed[g.Key] {
				continue
			}
		}
		items, depths := g.Items, make([]int, len(g.Items))
		if all != nil { // show hierarchy
			items, depths = nest(g.Items, all)
		}
		hideBelow := -1 // depth of a collapsed parent whose subtree is skipped
		for i, it := range items {
			d := depths[i]
			if hideBelow >= 0 && d > hideBelow {
				continue
			}
			hideBelow = -1
			children := 0
			for j := i + 1; j < len(items) && depths[j] > d; j++ {
				if depths[j] == d+1 {
					children++
				}
			}
			out = append(out, row{group: gi, item: it, depth: d, children: children})
			if children > 0 && !expanded[it.Ref()] {
				hideBelow = d
			}
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
