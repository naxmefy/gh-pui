package gh

import (
	"encoding/json"
	"testing"
)

const fixture = `{
  "id": "PVT_1", "number": 17, "title": "spike", "url": "https://github.com/users/x/projects/17",
  "fields": { "nodes": [
    { "id": "F1", "name": "Status", "dataType": "SINGLE_SELECT", "options": [{ "id": "o1", "name": "Todo", "color": "GRAY" }] },
    { "id": "F2", "name": "Area", "dataType": "MULTI_SELECT", "multiSelectOptions": [{ "id": "m1", "name": "ui", "color": "BLUE" }] },
    { "id": "F3", "name": "Sprint", "dataType": "ITERATION", "configuration": {
        "iterations": [{ "id": "i2", "title": "Sprint 2", "startDate": "2026-11-16", "duration": 14 }],
        "completedIterations": [{ "id": "i1", "title": "Sprint 1", "startDate": "2026-11-02", "duration": 14 }] } }
  ] },
  "views": { "nodes": [
    { "number": 1, "name": "Board", "layout": "BOARD_LAYOUT", "filter": "-status:Later",
      "groupByFields": { "nodes": [{ "name": "Parent issue" }] },
      "verticalGroupByFields": { "nodes": [{ "name": "Status" }] },
      "sortByFields": { "nodes": [{ "direction": "DESC", "field": { "name": "Estimate" } }] },
      "fields": { "nodes": [{ "name": "Title" }, {}] } }
  ] },
  "items": { "pageInfo": { "hasNextPage": false, "endCursor": "c" }, "nodes": [
    { "id": "I1", "type": "ISSUE", "isArchived": false,
      "content": { "__typename": "Issue", "title": "Fix it", "number": 7, "state": "OPEN", "url": "u", "repository": { "nameWithOwner": "x/y" } },
      "fieldValues": { "nodes": [
        { "__typename": "ProjectV2ItemFieldNumberValue", "number": 3.5, "field": { "name": "Estimate" } },
        { "__typename": "ProjectV2ItemFieldSingleSelectValue", "name": "Todo", "field": { "name": "Status" } },
        { "__typename": "ProjectV2ItemFieldMultiSelectValue", "value": "ui, api", "field": { "name": "Area" } },
        { "__typename": "ProjectV2ItemFieldIterationValue", "title": "Sprint 2", "field": { "name": "Sprint" } },
        { "__typename": "ProjectV2ItemFieldUserValue", "users": { "nodes": [{ "login": "a" }, { "login": "b" }] }, "field": { "name": "Assignees" } },
        { "__typename": "ProjectV2ItemFieldLabelValue", "labels": { "nodes": [{ "name": "bug" }] }, "field": { "name": "Labels" } },
        { "__typename": "ProjectV2ItemFieldRepositoryValue", "repository": { "nameWithOwner": "x/y" }, "field": { "name": "Repository" } },
        { "__typename": "ProjectV2ItemFieldReviewerValue" }
      ] } },
    { "id": "I2", "type": "DRAFT_ISSUE", "isArchived": true,
      "content": { "__typename": "DraftIssue", "title": "Idea" },
      "fieldValues": { "nodes": [] } }
  ] }
}`

func TestToProject(t *testing.T) {
	var raw rawProject
	if err := json.Unmarshal([]byte(fixture), &raw); err != nil {
		t.Fatal(err)
	}
	p := toProject("x", &raw)

	if p.Owner != "x" || p.Number != 17 || len(p.Fields) != 3 || len(p.Views) != 1 || len(p.Items) != 2 {
		t.Fatalf("unexpected shape: %+v", p)
	}
	if got := p.Fields[1].Options; len(got) != 1 || got[0].Name != "ui" {
		t.Errorf("multi-select options = %+v", got)
	}
	if its := p.Fields[2].Iterations; len(its) != 2 || its[0].Completed || !its[1].Completed {
		t.Errorf("iterations = %+v", its)
	}
	v := p.Views[0]
	if v.Filter != "-status:Later" || v.GroupBy[0] != "Parent issue" || v.VerticalGroupBy[0] != "Status" ||
		v.SortBy[0] != struct{ Field, Direction string }{"Estimate", "DESC"} || len(v.VisibleFields) != 1 {
		t.Errorf("view = %+v", v)
	}

	it := p.Items[0]
	if it.Title != "Fix it" || it.Number != 7 || it.Repo != "x/y" {
		t.Errorf("item = %+v", it)
	}
	want := map[string]string{"Estimate": "3.5", "Status": "Todo", "Area": "ui, api", "Sprint": "Sprint 2",
		"Assignees": "a, b", "Labels": "bug", "Repository": "x/y"}
	for k, w := range want {
		if got := it.Values[k].Text; got != w {
			t.Errorf("value %s = %q, want %q", k, got, w)
		}
	}
	if n := it.Values["Estimate"].Number; n == nil || *n != 3.5 {
		t.Errorf("estimate number = %v", n)
	}
	if len(it.Values) != len(want) {
		t.Errorf("unexpected extra values: %+v", it.Values)
	}
	if d := p.Items[1]; d.Type != "DRAFT_ISSUE" || !d.Archived || d.Title != "Idea" {
		t.Errorf("draft = %+v", d)
	}
}

func TestHasProjectScope(t *testing.T) {
	for header, want := range map[string]bool{
		"repo, read:org, project": true,
		"read:project":            true,
		"repo, read:org":          false,
		"read:projects":           false,
	} {
		if got := hasProjectScope(header); got != want {
			t.Errorf("%q: got %v", header, got)
		}
	}
}
