package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/naxmefy/gh-pui/internal/gh"
	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gh pui:", err)
		os.Exit(1)
	}
}

func run() error {
	owner := flag.String("owner", "", "project owner (user or org)")
	number := flag.Int("project", 0, "project number")
	flag.Parse()

	client, err := gh.New()
	if err != nil {
		return fmt.Errorf("not authenticated, run `gh auth login`: %w", err)
	}
	if err := client.CheckScopes(); err != nil {
		return err
	}
	login, err := client.Viewer()
	if err != nil {
		return err
	}

	if *owner == "" || *number == 0 {
		if *owner, *number, err = projectFromRepo(client); err != nil {
			return err
		}
	}

	p, err := client.LoadProject(*owner, *number)
	if err != nil {
		return err
	}
	if err := project.Save(p); err != nil {
		fmt.Fprintln(os.Stderr, "gh pui: cache not written:", err)
	}

	_, err = tea.NewProgram(ui.NewModel(login, p)).Run()
	return err
}

// shortcut: picks the repo's single linked project; replace with the project picker (phase 1a startup logic).
func projectFromRepo(client *gh.Client) (string, int, error) {
	repo := gh.CurrentRepo()
	if repo == "" {
		return "", 0, fmt.Errorf("not in a GitHub repo, pass --owner and --project")
	}
	refs, err := client.RepoProjects(repo)
	if err != nil {
		return "", 0, err
	}
	var open []gh.ProjectRef
	for _, r := range refs {
		if !r.Closed {
			open = append(open, r)
		}
	}
	if len(open) == 1 {
		return open[0].Owner, open[0].Number, nil
	}
	var list []string
	for _, r := range open {
		list = append(list, fmt.Sprintf("  --owner %s --project %d  (%s)", r.Owner, r.Number, r.Title))
	}
	return "", 0, fmt.Errorf("%s has %d linked projects, pass one of:\n%s", repo, len(open), strings.Join(list, "\n"))
}
