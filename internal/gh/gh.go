// Package gh is the only place that talks to GitHub; it maps API responses to the project domain model.
package gh

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/naxmefy/gh-pui/internal/project"
)

type Client struct {
	gql  *api.GraphQLClient
	rest *api.RESTClient
}

// New uses the token and host of the gh login.
func New() (*Client, error) {
	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, err
	}
	rest, err := api.DefaultRESTClient()
	if err != nil {
		return nil, err
	}
	return &Client{gql: gql, rest: rest}, nil
}

func (c *Client) Viewer() (string, error) {
	var resp struct{ Viewer struct{ Login string } }
	err := c.gql.Do(`{ viewer { login } }`, nil, &resp)
	return resp.Viewer.Login, err
}

// ErrMissingScope means the token cannot read projects; fix with `gh auth refresh -s project`.
var ErrMissingScope = errors.New("token lacks the project scope, run `gh auth refresh -s project`")

// CheckScopes fails only for classic tokens that list their scopes and lack project access;
// fine-grained and app tokens send no scope header and are left to fail on the actual query.
func (c *Client) CheckScopes() error {
	resp, err := c.rest.Request("GET", "user", nil)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	header := resp.Header.Get("X-Oauth-Scopes")
	if header == "" {
		return nil
	}
	if !hasProjectScope(header) {
		return ErrMissingScope
	}
	return nil
}

func hasProjectScope(header string) bool {
	for _, s := range strings.Split(header, ",") {
		if s = strings.TrimSpace(s); s == "project" || s == "read:project" {
			return true
		}
	}
	return false
}

// CurrentRepo returns "owner/name" of the git repo in the working directory, or "" outside a GitHub repo.
func CurrentRepo() string {
	r, err := repository.Current()
	if err != nil {
		return ""
	}
	return r.Owner + "/" + r.Name
}

// ProjectRef is a project in a list, without its items.
type ProjectRef struct {
	Owner  string
	Number int
	Title  string
	Closed bool
}

const refFields = `nodes { number title closed owner { ... on User { login } ... on Organization { login } } }`

type rawRef struct {
	Number int
	Title  string
	Closed bool
	Owner  struct{ Login string }
}

func toRefs(nodes []rawRef) []ProjectRef {
	refs := make([]ProjectRef, len(nodes))
	for i, n := range nodes {
		refs[i] = ProjectRef{Owner: n.Owner.Login, Number: n.Number, Title: n.Title, Closed: n.Closed}
	}
	return refs
}

// RepoProjects lists the projects linked to a repository ("owner/name").
func (c *Client) RepoProjects(repo string) ([]ProjectRef, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return nil, fmt.Errorf("invalid repo %q", repo)
	}
	var resp struct {
		Repository struct{ ProjectsV2 struct{ Nodes []rawRef } }
	}
	q := `query($owner:String!,$name:String!){ repository(owner:$owner,name:$name){ projectsV2(first:100){ ` + refFields + ` } } }`
	err := c.gql.Do(q, map[string]interface{}{"owner": owner, "name": name}, &resp)
	return toRefs(resp.Repository.ProjectsV2.Nodes), err
}

// OwnerProjects lists the projects of a user or organization.
func (c *Client) OwnerProjects(login string) ([]ProjectRef, error) {
	var resp struct {
		RepositoryOwner struct{ ProjectsV2 struct{ Nodes []rawRef } }
	}
	q := `query($login:String!){ repositoryOwner(login:$login){ ... on ProjectV2Owner { projectsV2(first:100){ ` + refFields + ` } } } }`
	err := c.gql.Do(q, map[string]interface{}{"login": login}, &resp)
	return toRefs(resp.RepositoryOwner.ProjectsV2.Nodes), err
}

const fieldName = `field { ... on ProjectV2FieldCommon { name } }`

// projectQuery loads fields and views only on the first page ($meta) and items in pages of 100.
const projectQuery = `query($owner:String!,$number:Int!,$after:String,$meta:Boolean!){
  repositoryOwner(login:$owner){ ... on ProjectV2Owner { projectV2(number:$number){
    id number title url
    fields(first:100) @include(if:$meta) { nodes {
      ... on ProjectV2FieldCommon { id name dataType }
      ... on ProjectV2SingleSelectField { options { id name color } }
      ... on ProjectV2MultiSelectField { multiSelectOptions { id name color } }
      ... on ProjectV2IterationField { configuration {
        iterations { id title startDate duration }
        completedIterations { id title startDate duration } } }
    } }
    views(first:50) @include(if:$meta) { nodes { number name layout filter
      groupByFields(first:5) { nodes { ... on ProjectV2FieldCommon { name } } }
      verticalGroupByFields(first:5) { nodes { ... on ProjectV2FieldCommon { name } } }
      sortByFields(first:5) { nodes { direction field { ... on ProjectV2FieldCommon { name } } } }
      fields(first:50) { nodes { ... on ProjectV2FieldCommon { name } } }
    } }
    items(first:100, after:$after, orderBy:{field:POSITION, direction:ASC}) { pageInfo { hasNextPage endCursor } nodes {
      id type isArchived
      content { __typename
        ... on DraftIssue { title }
        ... on Issue { title number state url repository { nameWithOwner }
          parent { number title repository { nameWithOwner } } subIssuesSummary { completed total } }
        ... on PullRequest { title number state url repository { nameWithOwner } }
      }
      fieldValues(first:50) { nodes { __typename
        ... on ProjectV2ItemFieldTextValue { text ` + fieldName + ` }
        ... on ProjectV2ItemFieldNumberValue { number ` + fieldName + ` }
        ... on ProjectV2ItemFieldDateValue { date ` + fieldName + ` }
        ... on ProjectV2ItemFieldSingleSelectValue { name ` + fieldName + ` }
        ... on ProjectV2ItemFieldMultiSelectValue { value ` + fieldName + ` }
        ... on ProjectV2ItemFieldIterationValue { title ` + fieldName + ` }
        ... on ProjectV2ItemFieldUserValue { users(first:10) { nodes { login } } ` + fieldName + ` }
        ... on ProjectV2ItemFieldLabelValue { labels(first:20) { nodes { name } } ` + fieldName + ` }
        ... on ProjectV2ItemFieldMilestoneValue { milestone { title } ` + fieldName + ` }
        ... on ProjectV2ItemFieldRepositoryValue { repository { nameWithOwner } ` + fieldName + ` }
      } }
    } }
  } } }
}`

type named struct{ Name string }

type rawIteration struct {
	ID, Title, StartDate string
	Duration             int
}

type rawOption struct{ ID, Name, Color string }

type rawProject struct {
	ID     string
	Number int
	Title  string
	URL    string
	Fields struct {
		Nodes []struct {
			ID, Name, DataType string
			Options            []rawOption
			MultiSelectOptions []rawOption
			Configuration      *struct {
				Iterations          []rawIteration
				CompletedIterations []rawIteration
			}
		}
	}
	Views struct {
		Nodes []struct {
			Number                int
			Name, Layout, Filter  string
			GroupByFields         struct{ Nodes []named }
			VerticalGroupByFields struct{ Nodes []named }
			SortByFields          struct {
				Nodes []struct {
					Direction string
					Field     named
				}
			}
			Fields struct{ Nodes []named }
		}
	}
	Items struct {
		PageInfo struct {
			HasNextPage bool
			EndCursor   string
		}
		Nodes []rawItem
	}
}

type rawItem struct {
	ID         string
	Type       string
	IsArchived bool
	Content    *struct {
		Title, State, URL string
		Number            int
		Repository        *struct{ NameWithOwner string }
		Parent            *struct {
			Number     int
			Title      string
			Repository struct{ NameWithOwner string }
		}
		SubIssuesSummary *struct{ Completed, Total int }
	}
	FieldValues struct {
		Nodes []rawValue
	}
}

type rawValue struct {
	Typename  string `json:"__typename"`
	Field     named
	Text      string
	Number    *float64
	Date      string
	Name      string
	Value     string
	Title     string
	Users     *struct{ Nodes []struct{ Login string } }
	Labels    *struct{ Nodes []named }
	Milestone *struct{ Title string }
	Repo      *struct{ NameWithOwner string } `json:"repository"`
}

// LoadProject fetches a project with all its items (paging through them).
func (c *Client) LoadProject(owner string, number int) (*project.Project, error) {
	var p *project.Project
	vars := map[string]interface{}{"owner": owner, "number": number, "after": (*string)(nil), "meta": true}
	for {
		var resp struct {
			RepositoryOwner *struct{ ProjectV2 *rawProject }
		}
		if err := c.gql.Do(projectQuery, vars, &resp); err != nil {
			return nil, err
		}
		if resp.RepositoryOwner == nil || resp.RepositoryOwner.ProjectV2 == nil {
			return nil, fmt.Errorf("project %s/%d not found", owner, number)
		}
		raw := resp.RepositoryOwner.ProjectV2
		if p == nil {
			p = toProject(owner, raw)
		} else {
			p.Items = append(p.Items, toItems(raw.Items.Nodes)...)
		}
		if !raw.Items.PageInfo.HasNextPage {
			return p, c.loadViewItems(p)
		}
		vars["after"] = raw.Items.PageInfo.EndCursor
		vars["meta"] = false
	}
}

func toProject(owner string, raw *rawProject) *project.Project {
	p := &project.Project{ID: raw.ID, Owner: owner, Number: raw.Number, Title: raw.Title, URL: raw.URL}
	for _, f := range raw.Fields.Nodes {
		field := project.Field{ID: f.ID, Name: f.Name, DataType: f.DataType}
		for _, o := range append(f.Options, f.MultiSelectOptions...) {
			field.Options = append(field.Options, project.Option{ID: o.ID, Name: o.Name, Color: o.Color})
		}
		if f.Configuration != nil {
			for _, it := range f.Configuration.Iterations {
				field.Iterations = append(field.Iterations, project.Iteration{ID: it.ID, Title: it.Title, StartDate: it.StartDate, Duration: it.Duration})
			}
			for _, it := range f.Configuration.CompletedIterations {
				field.Iterations = append(field.Iterations, project.Iteration{ID: it.ID, Title: it.Title, StartDate: it.StartDate, Duration: it.Duration, Completed: true})
			}
		}
		p.Fields = append(p.Fields, field)
	}
	for _, v := range raw.Views.Nodes {
		view := project.View{Number: v.Number, Name: v.Name, Layout: v.Layout, Filter: v.Filter,
			GroupBy: names(v.GroupByFields.Nodes), VerticalGroupBy: names(v.VerticalGroupByFields.Nodes), VisibleFields: names(v.Fields.Nodes)}
		for _, s := range v.SortByFields.Nodes {
			view.SortBy = append(view.SortBy, project.Sort{Field: s.Field.Name, Direction: s.Direction})
		}
		p.Views = append(p.Views, view)
	}
	p.Items = toItems(raw.Items.Nodes)
	return p
}

func names(ns []named) []string {
	out := make([]string, 0, len(ns))
	for _, n := range ns {
		if n.Name != "" {
			out = append(out, n.Name)
		}
	}
	return out
}

func toItems(raw []rawItem) []project.Item {
	items := make([]project.Item, 0, len(raw))
	for _, r := range raw {
		it := project.Item{ID: r.ID, Type: r.Type, Archived: r.IsArchived, Values: map[string]project.Value{}}
		if c := r.Content; c != nil {
			it.Title, it.State, it.URL, it.Number = c.Title, c.State, c.URL, c.Number
			if c.Repository != nil {
				it.Repo = c.Repository.NameWithOwner
			}
			// parent and sub-issue progress are not delivered as field values, so fill those fields from the issue itself
			if c.Parent != nil {
				it.Values["Parent issue"] = project.Value{Text: fmt.Sprintf("#%d %s", c.Parent.Number, c.Parent.Title)}
				it.Parent = project.Ref(c.Parent.Repository.NameWithOwner, c.Parent.Number)
			}
			if c.SubIssuesSummary != nil && c.SubIssuesSummary.Total > 0 {
				it.Values["Sub-issues progress"] = project.Value{Text: fmt.Sprintf("%d/%d", c.SubIssuesSummary.Completed, c.SubIssuesSummary.Total)}
			}
		}
		for _, v := range r.FieldValues.Nodes {
			if v.Field.Name == "" {
				continue // value types we do not query (e.g. reviewers) arrive empty
			}
			it.Values[v.Field.Name] = toValue(v)
		}
		items = append(items, it)
	}
	return items
}

func toValue(v rawValue) project.Value {
	switch v.Typename {
	case "ProjectV2ItemFieldNumberValue":
		if v.Number == nil {
			return project.Value{}
		}
		return project.Value{Text: strconv.FormatFloat(*v.Number, 'f', -1, 64), Number: v.Number}
	case "ProjectV2ItemFieldTextValue":
		return project.Value{Text: v.Text}
	case "ProjectV2ItemFieldDateValue":
		return project.Value{Text: v.Date}
	case "ProjectV2ItemFieldSingleSelectValue":
		return project.Value{Text: v.Name}
	case "ProjectV2ItemFieldMultiSelectValue":
		return project.Value{Text: v.Value}
	case "ProjectV2ItemFieldIterationValue":
		return project.Value{Text: v.Title}
	case "ProjectV2ItemFieldUserValue":
		var ls []string
		if v.Users != nil {
			for _, u := range v.Users.Nodes {
				ls = append(ls, u.Login)
			}
		}
		return project.Value{Text: strings.Join(ls, ", ")}
	case "ProjectV2ItemFieldLabelValue":
		if v.Labels == nil {
			return project.Value{}
		}
		return project.Value{Text: strings.Join(names(v.Labels.Nodes), ", ")}
	case "ProjectV2ItemFieldMilestoneValue":
		if v.Milestone == nil {
			return project.Value{}
		}
		return project.Value{Text: v.Milestone.Title}
	case "ProjectV2ItemFieldRepositoryValue":
		if v.Repo == nil {
			return project.Value{}
		}
		return project.Value{Text: v.Repo.NameWithOwner}
	}
	return project.Value{}
}

// loadViewItems stores which items match each view's saved filter; the server evaluates the filter syntax.
func (c *Client) loadViewItems(p *project.Project) error {
	const q = `query($id:ID!,$query:String!,$after:String){ node(id:$id){ ... on ProjectV2 {
  items(first:100, after:$after, query:$query) { pageInfo { hasNextPage endCursor } nodes { id } } } } }`
	for i := range p.Views {
		v := &p.Views[i]
		if v.Filter == "" {
			continue
		}
		v.ItemIDs = []string{}
		vars := map[string]interface{}{"id": p.ID, "query": v.Filter, "after": (*string)(nil)}
		for {
			var resp struct {
				Node struct {
					Items struct {
						PageInfo struct {
							HasNextPage bool
							EndCursor   string
						}
						Nodes []struct{ ID string }
					}
				}
			}
			if err := c.gql.Do(q, vars, &resp); err != nil {
				return fmt.Errorf("view %q filter: %w", v.Name, err)
			}
			for _, n := range resp.Node.Items.Nodes {
				v.ItemIDs = append(v.ItemIDs, n.ID)
			}
			if !resp.Node.Items.PageInfo.HasNextPage {
				break
			}
			vars["after"] = resp.Node.Items.PageInfo.EndCursor
		}
	}
	return nil
}

// Orgs lists the logins of the viewer's organizations (empty without the read:org scope).
func (c *Client) Orgs() ([]string, error) {
	var resp struct {
		Viewer struct {
			Organizations struct{ Nodes []struct{ Login string } }
		}
	}
	err := c.gql.Do(`{ viewer { organizations(first:100) { nodes { login } } } }`, nil, &resp)
	var out []string
	for _, n := range resp.Viewer.Organizations.Nodes {
		out = append(out, n.Login)
	}
	return out, err
}

// ViewerProjects lists the viewer's own projects and those of the viewer's organizations in one query.
// An org that refuses access (e.g. SAML) yields a GraphQL error; the projects that did load are still returned.
func (c *Client) ViewerProjects() ([]ProjectRef, error) {
	var resp struct {
		Viewer struct {
			ProjectsV2    struct{ Nodes []rawRef }
			Organizations struct {
				Nodes []struct{ ProjectsV2 struct{ Nodes []rawRef } }
			}
		}
	}
	err := c.gql.Do(`{ viewer { projectsV2(first:100){ `+refFields+` }
  organizations(first:100){ nodes { projectsV2(first:100){ `+refFields+` } } } } }`, nil, &resp)
	nodes := resp.Viewer.ProjectsV2.Nodes
	for _, o := range resp.Viewer.Organizations.Nodes {
		nodes = append(nodes, o.ProjectsV2.Nodes...)
	}
	return toRefs(nodes), err
}

// FieldValue is a new value for a project field: set one member, or none to clear the field.
type FieldValue struct {
	Text        *string
	Number      *float64
	Date        string // YYYY-MM-DD
	OptionID    string
	OptionIDs   []string
	IterationID string
}

func (v FieldValue) input() map[string]interface{} {
	switch {
	case v.Text != nil:
		return map[string]interface{}{"text": *v.Text}
	case v.Number != nil:
		return map[string]interface{}{"number": *v.Number}
	case v.Date != "":
		return map[string]interface{}{"date": v.Date}
	case v.OptionID != "":
		return map[string]interface{}{"singleSelectOptionId": v.OptionID}
	case len(v.OptionIDs) > 0:
		return map[string]interface{}{"multiSelectOptionIds": v.OptionIDs}
	case v.IterationID != "":
		return map[string]interface{}{"iterationId": v.IterationID}
	}
	return nil
}

// UpdateField sets (or clears, for an empty FieldValue) one field of a project item.
func (c *Client) UpdateField(projectID, itemID, fieldID string, v FieldValue) error {
	input := map[string]interface{}{"projectId": projectID, "itemId": itemID, "fieldId": fieldID}
	q := `mutation($input:ClearProjectV2ItemFieldValueInput!){ clearProjectV2ItemFieldValue(input:$input){ clientMutationId } }`
	if value := v.input(); value != nil {
		input["value"] = value
		q = `mutation($input:UpdateProjectV2ItemFieldValueInput!){ updateProjectV2ItemFieldValue(input:$input){ clientMutationId } }`
	}
	var resp struct{}
	return c.gql.Do(q, map[string]interface{}{"input": input}, &resp)
}
