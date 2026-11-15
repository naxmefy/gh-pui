package ui

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/naxmefy/gh-pui/internal/config"
	"github.com/naxmefy/gh-pui/internal/gh"
	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui/theme"
)

// Run `go test ./internal/ui -update` after an intended layout change and review the diff of testdata/.
var update = flag.Bool("update", false, "rewrite golden files")

// widths hits every breakpoint: xs <60, sm <100, md <140, lg <180, xl.
var widths = []struct {
	bp    string
	width int
}{{"xs", 50}, {"sm", 80}, {"md", 120}, {"lg", 160}, {"xl", 200}}

func requireGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if string(want) != got {
		t.Errorf("%s differs from the golden file; run with -update if intended.\n--- want\n%s\n--- got\n%s", path, want, got)
	}
}

func goldenProject() *project.Project {
	v := func(s string) project.Value { return project.Value{Text: s} }
	n := func(f float64, s string) project.Value { return project.Value{Text: s, Number: &f} }
	issue := func(id string, num int, title, state string, vals map[string]project.Value) project.Item {
		return project.Item{ID: id, Type: "ISSUE", Number: num, Title: title, State: state, Repo: "acme/app", Values: vals}
	}
	return &project.Project{
		Owner: "acme", Number: 7, Title: "App roadmap",
		Fields: []project.Field{
			{Name: "Status", DataType: "SINGLE_SELECT", Options: []project.Option{{Name: "Todo"}, {Name: "In Progress"}, {Name: "Done"}}},
			{Name: "Estimate", DataType: "NUMBER"},
		},
		Views: []project.View{
			{Number: 1, Name: "Backlog by status", GroupBy: []string{"Status"},
				VisibleFields: []string{"Title", "Assignees", "Status", "Labels", "Estimate", "Milestone", "Repository", "Linked pull requests"}},
			{Number: 2, Name: "Board", Layout: "BOARD_LAYOUT"},
			{Number: 3, Name: "Roadmap", Layout: "ROADMAP_LAYOUT"},
		},
		Items: []project.Item{
			issue("a", 12, "Login with passkeys", "OPEN", map[string]project.Value{"Status": v("In Progress"), "Assignees": v("mona"), "Labels": v("auth, feature"), "Estimate": n(5, "5"), "Milestone": v("v1.0"), "Repository": v("acme/app"), "Parent issue": v("#3 Accounts"), "Sub-issues progress": v("1/3")}),
			issue("b", 15, "Crash when the cache directory is read-only", "OPEN", map[string]project.Value{"Status": v("Todo"), "Assignees": v("hubot"), "Labels": v("bug"), "Estimate": n(2, "2"), "Repository": v("acme/app")}),
			{ID: "c", Type: "DRAFT_ISSUE", Title: "Sketch the onboarding flow", Values: map[string]project.Value{"Status": v("Todo"), "Estimate": n(3, "3")}},
			{ID: "d", Type: "PULL_REQUEST", Number: 18, Title: "Add passkey endpoints", State: "MERGED", Repo: "acme/app", Values: map[string]project.Value{"Status": v("Done"), "Assignees": v("mona"), "Repository": v("acme/app"), "Linked pull requests": v("#12")}},
			issue("e", 9, "Document the release process", "CLOSED", map[string]project.Value{"Status": v("Done"), "Labels": v("docs"), "Estimate": n(1, "1"), "Milestone": v("v0.9"), "Repository": v("acme/app")}),
			issue("f", 21, "Settings page", "OPEN", map[string]project.Value{"Repository": v("acme/app")}),
		},
	}
}

func goldenRefs() []gh.ProjectRef {
	return []gh.ProjectRef{{Owner: "acme", Number: 7, Title: "App roadmap"}, {Owner: "acme", Number: 2, Title: "Support queue"}, {Owner: "mona", Number: 1, Title: "Personal tasks"}}
}

func TestGoldenScreens(t *testing.T) {
	th, _ := theme.New(config.Default(), "none")
	keys, _ := NewKeyMap(nil)
	load := func(gh.ProjectRef, bool) (*project.Project, error) { return goldenProject(), nil }
	source := func() ([]gh.ProjectRef, []gh.ProjectRef, error) { return goldenRefs(), goldenRefs()[:1], nil }
	lister := func(string) ([]gh.ProjectRef, error) { return goldenRefs(), nil }

	for _, w := range widths {
		size := tea.WindowSizeMsg{Width: w.width, Height: 24}
		screens := map[string]tea.Model{}

		m, _ := NewModel("mona", goldenProject(), th, keys).Update(size)
		screens["table"] = drive(m, "down") // cursor on an item, so the peek is filled

		var app tea.Model = NewApp("mona", th, keys, NewPicker(th, lister, Page{Title: "Choose an owner", Subtitle: "Not in a GitHub repo", Entries: OwnerEntries("mona", "acme")}), load, source, nil, nil)
		app, _ = app.Update(size)
		screens["picker-owners"] = app
		screens["picker-projects"] = drive(app, "down", "enter")
		screens["switcher"] = drive(app, "down", "enter", "enter", "ctrl+p")

		for name, s := range screens {
			t.Run(name+"-"+w.bp, func(t *testing.T) {
				requireGolden(t, name+"-"+w.bp, ansi.Strip(s.View().Content)+"\n")
			})
		}
	}
}
