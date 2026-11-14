package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/naxmefy/gh-pui/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gh pui:", err)
		os.Exit(1)
	}
}

func run() error {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return fmt.Errorf("not authenticated, run `gh auth login`: %w", err)
	}
	var query struct {
		Viewer struct{ Login string }
	}
	if err := client.Query("Viewer", &query, nil); err != nil {
		return err
	}

	_, err = tea.NewProgram(ui.NewModel(query.Viewer.Login)).Run()
	return err
}
