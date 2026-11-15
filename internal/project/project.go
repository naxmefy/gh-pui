// Package project holds the domain model of a GitHub Project (v2), independent of the API wire format.
package project

type Project struct {
	ID     string
	Owner  string
	Number int
	Title  string
	URL    string
	Fields []Field
	Views  []View
	Items  []Item
}

type Field struct {
	ID         string
	Name       string
	DataType   string // TEXT, NUMBER, DATE, SINGLE_SELECT, MULTI_SELECT, ITERATION, ASSIGNEES, LABELS, ...
	Options    []Option
	Iterations []Iteration
}

type Option struct {
	ID    string
	Name  string
	Color string
}

type Iteration struct {
	ID        string
	Title     string
	StartDate string
	Duration  int
	Completed bool
}

type View struct {
	Number          int
	Name            string
	Layout          string // TABLE_LAYOUT, BOARD_LAYOUT, ROADMAP_LAYOUT
	Filter          string
	GroupBy         []string
	VerticalGroupBy []string
	SortBy          []Sort
	VisibleFields   []string
	ItemIDs         []string // items matching Filter; nil when the view has no filter
}

type Sort struct {
	Field     string
	Direction string // ASC, DESC
}

type Item struct {
	ID       string
	Type     string // ISSUE, PULL_REQUEST, DRAFT_ISSUE, REDACTED
	Title    string
	Number   int
	State    string
	URL      string
	Repo     string
	Archived bool
	Values   map[string]Value // by field name
}

// Value is a field value ready for display; Number is set only for NUMBER fields so sums can be computed.
type Value struct {
	Text   string
	Number *float64
}
