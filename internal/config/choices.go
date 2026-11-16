package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Projects is projects.yml: the project remembered per repo and local settings per project,
// for view options the GitHub API does not expose.
//
//	repos:
//	  acme/app: {owner: acme, number: 7}
//	projects:
//	  acme/7:
//	    sum: [Estimate]          # fields summed in group headers; [] = count only
//	    views:
//	      Backlog: {hierarchy: false, slice: Priority}
type Projects struct {
	Repos    map[string]Choice          `yaml:"repos"`
	Projects map[string]ProjectSettings `yaml:"projects"` // key "owner/number"
}

// Choice is the project picked for a repo, so the picker is skipped next time.
type Choice struct {
	Owner  string `yaml:"owner"`
	Number int    `yaml:"number"`
}

type ProjectSettings struct {
	Sum   *[]string               `yaml:"sum"`   // nil: the project's estimation field
	Views map[string]ViewSettings `yaml:"views"` // by view name
}

type ViewSettings struct {
	Hierarchy *bool  `yaml:"hierarchy"` // nil: on, like github.com
	Slice     string `yaml:"slice"`     // field name, "" = no slicing
}

// Settings returns the settings of one project (zero value when none are configured).
func (p Projects) Settings(owner string, number int) ProjectSettings {
	return p.Projects[fmt.Sprintf("%s/%d", owner, number)]
}

func projectsPath() (string, error) {
	dir, err := Dir()
	return filepath.Join(dir, "projects.yml"), err
}

// LoadProjects reads projects.yml; a missing file is empty. A file in the first format
// (a flat map repo -> project) is read as repos.
func LoadProjects() (Projects, error) {
	path, err := projectsPath()
	if err != nil {
		return Projects{}, err
	}
	var p Projects
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	var probe map[string]any
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return p, fmt.Errorf("%s: %w", path, err)
	}
	_, hasRepos := probe["repos"]
	_, hasProjects := probe["projects"]
	if len(probe) > 0 && !hasRepos && !hasProjects {
		err = yaml.Unmarshal(data, &p.Repos)
	} else {
		err = yaml.Unmarshal(data, &p)
	}
	if err != nil {
		return p, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

// SaveChoice remembers the project for a repo; the file is rewritten whole, so comments in it are lost.
func SaveChoice(repo string, c Choice) error {
	p, err := LoadProjects()
	if err != nil {
		return err
	}
	if p.Repos[repo] == c {
		return nil
	}
	if p.Repos == nil {
		p.Repos = map[string]Choice{}
	}
	p.Repos[repo] = c
	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	path, err := projectsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
