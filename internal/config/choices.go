package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Projects is projects.yml: the project remembered per repo and local settings per project,
// for view options the GitHub API does not expose.
//
//	repos:
//	  acme/app: acme/7           # repo -> project (owner/number) opened for it
//	projects:
//	  acme/7:
//	    sum: [Estimate]          # fields summed in group headers (default none)
//	    count: false             # hide the item count (default shown)
//	    views:                   # per view, overriding the project's values
//	      Backlog: {hierarchy: false, slice: Priority, sum: []}
type Projects struct {
	Repos    map[string]Choice          `yaml:"repos,omitempty"`
	Projects map[string]ProjectSettings `yaml:"projects,omitempty"` // key "owner/number"
}

// Choice is the project picked for a repo, so the picker is skipped next time.
// In projects.yml it is written as "owner/number"; the earlier {owner, number} mapping is still read.
type Choice struct {
	Owner  string `yaml:"owner"`
	Number int    `yaml:"number"`
}

func (c Choice) MarshalYAML() (any, error) { return fmt.Sprintf("%s/%d", c.Owner, c.Number), nil }

func (c *Choice) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.MappingNode {
		type plain Choice // no UnmarshalYAML, avoids recursion
		return n.Decode((*plain)(c))
	}
	owner, num, ok := strings.Cut(n.Value, "/")
	number, err := strconv.Atoi(num)
	if !ok || owner == "" || err != nil {
		return fmt.Errorf("line %d: %q is not owner/number", n.Line, n.Value)
	}
	*c = Choice{Owner: owner, Number: number}
	return nil
}

type ProjectSettings struct {
	Sum   *[]string               `yaml:"sum"`   // number fields summed in group headers; nil = none
	Count *bool                   `yaml:"count"` // nil: shown
	Views map[string]ViewSettings `yaml:"views"` // by view name
}

type ViewSettings struct {
	Sum       *[]string `yaml:"sum"`       // nil: the project's
	Count     *bool     `yaml:"count"`     // nil: the project's
	Hierarchy *bool     `yaml:"hierarchy"` // nil: on, like github.com
	Slice     string    `yaml:"slice"`     // field name, "" = no slicing
}

// SumFor and CountFor resolve a view's field sum: the view's setting, else the project's, else the default.
func (s ProjectSettings) SumFor(view string) *[]string {
	if v := s.Views[view].Sum; v != nil {
		return v
	}
	return s.Sum
}

func (s ProjectSettings) CountFor(view string) bool {
	if v := s.Views[view].Count; v != nil {
		return *v
	}
	return s.Count == nil || *s.Count
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
