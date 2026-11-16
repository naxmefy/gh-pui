package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/naxmefy/gh-pui/internal/config"
	"github.com/naxmefy/gh-pui/internal/gh"
	"github.com/naxmefy/gh-pui/internal/project"
	"github.com/naxmefy/gh-pui/internal/ui"
	"github.com/naxmefy/gh-pui/internal/ui/theme"
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
	themeFlag := flag.String("theme", "", "auto | ansi | none (default from config.yml)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	th, err := theme.New(cfg, *themeFlag)
	if err != nil {
		return err
	}
	keys, err := ui.NewKeyMap(cfg.Keys)
	if err != nil {
		return err
	}

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

	repo := gh.CurrentRepo()
	var warns []string
	var mu sync.Mutex
	warn := func(format string, a ...any) {
		mu.Lock()
		defer mu.Unlock()
		warns = append(warns, fmt.Sprintf(format, a...))
	}
	load := func(ref gh.ProjectRef, picked bool) (*project.Project, error) {
		p, err := client.LoadProject(ref.Owner, ref.Number)
		if err != nil {
			return nil, err
		}
		if err := project.Save(p); err != nil {
			warn("cache not written: %v", err)
		}
		if picked && repo != "" {
			if err := config.SaveChoice(repo, config.Choice{Owner: ref.Owner, Number: ref.Number}); err != nil {
				warn("choice not remembered: %v", err)
			}
		}
		return p, nil
	}

	first, auto, err := startPage(client, login, repo, *owner, *number)
	if err != nil {
		return err
	}
	source := func() ([]gh.ProjectRef, []gh.ProjectRef, error) {
		viewer, err := client.ViewerProjects()
		if repo == "" {
			return viewer, nil, err
		}
		inRepo, rerr := client.RepoProjects(repo)
		return viewer, inRepo, errors.Join(err, rerr)
	}
	projects, err := config.LoadProjects()
	if err != nil {
		return err
	}
	app := ui.NewApp(login, th, keys, ui.NewPicker(th, client.OwnerProjects, first), load, source, client.UpdateField, auto).
		WithSettings(projects.Settings)
	_, err = tea.NewProgram(app).Run()
	mu.Lock()
	defer mu.Unlock()
	for _, w := range warns {
		fmt.Fprintln(os.Stderr, "gh pui:", w)
	}
	return err
}

// startPage decides the first screen: a project to open right away (auto), or the picker page to start on.
func startPage(client *gh.Client, login, repo, owner string, number int) (ui.Page, *gh.ProjectRef, error) {
	switch {
	case owner != "" && number != 0:
		return ui.Page{Title: "Choose a project"}, &gh.ProjectRef{Owner: owner, Number: number}, nil
	case owner != "":
		refs, err := client.OwnerProjects(owner)
		entries := ui.ProjectEntries(refs)
		return ui.Page{Title: "Projects of " + owner, Subtitle: fmt.Sprintf("%d open projects", len(entries)), Entries: entries}, nil, err
	case repo == "":
		return ui.Page{Title: "Choose an owner", Subtitle: "Not in a GitHub repo", Entries: ui.OwnerEntries(append([]string{login}, orgs(client)...)...)}, nil, nil
	}

	refs, err := client.RepoProjects(repo)
	if err != nil {
		return ui.Page{}, nil, err
	}
	entries := ui.ProjectEntries(refs)
	projects, err := config.LoadProjects()
	if err != nil {
		return ui.Page{}, nil, err
	}
	choices := projects.Repos
	for _, e := range entries { // a remembered project counts only while it is still linked and open
		if c := choices[repo]; c.Owner == e.Ref.Owner && c.Number == e.Ref.Number {
			return ui.Page{Title: repo, Entries: entries}, e.Ref, nil
		}
	}
	switch len(entries) {
	case 0:
		// shortcut: linking the repo and creating a project are planned for v0.6; browsing owners covers the rest.
		repoOwner, _, _ := strings.Cut(repo, "/")
		owners := ui.OwnerEntries(append([]string{repoOwner, login}, orgs(client)...)...)
		return ui.Page{Title: "Choose an owner", Subtitle: repo + " has no linked project", Entries: owners}, nil, nil
	case 1:
		return ui.Page{Title: repo, Entries: entries}, entries[0].Ref, nil
	}
	return ui.Page{Title: repo, Subtitle: fmt.Sprintf("%d linked projects · your choice is remembered", len(entries)), Entries: entries}, nil, nil
}

// orgs is best effort: without read:org the picker just offers fewer owners.
func orgs(client *gh.Client) []string {
	o, _ := client.Orgs()
	return o
}
