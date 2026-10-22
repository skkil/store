package profile

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type fixedEnv struct{}

func (fixedEnv) NewID(n uint32) string   { return []string{"id-0", "id-1"}[n] }
func (fixedEnv) Row(table string) string { return "row-of-" + table }
func (fixedEnv) Text() string            { return "some words" }
func (fixedEnv) Int() int64              { return 42 }
func (fixedEnv) Bool() bool              { return true }
func (fixedEnv) ClientID() string        { return "client" }
func (fixedEnv) MutationID() uint64      { return 7 }

const valid = `{
  "schema_version": 3,
  "mutations": [
    {"name": "a.create", "kind": "create", "args": {"id": "{{new_id}}", "child": "{{new_id:1}}", "n": "{{int}}", "on": "{{bool}}", "m": "{{mutation_id}}",
      "label": "{{text}} #{{int}} by {{client_id}}", "tags": ["{{text}}"]}, "creates": ["a", "b"]},
    {"name": "a.update", "kind": "update", "weight": 3, "args": {"id": "{{row:a}}"}},
    {"name": "a.create", "kind": "reject", "weight": 0, "args": {"title": ""}},
    {"name": "a.bad", "kind": "reject", "args": {"title": ""}}
  ]
}`

func TestParseAndRender(t *testing.T) {
	p, err := Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if p.SchemaVersion != 3 || len(p.Mutations) != 4 {
		t.Fatalf("parsed %+v", p)
	}
	create := p.Mutations[0]
	if create.Weight != 1 || len(create.Tables()) != 0 {
		t.Fatalf("create = weight %d tables %v", create.Weight, create.Tables())
	}
	raw, err := create.Render(fixedEnv{})
	if err != nil {
		t.Fatal(err)
	}
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"id": "id-0", "child": "id-1", "n": float64(42), "on": true, "m": float64(7),
		"label": "some words #42 by client", "tags": []any{"some words"},
	}
	for k, v := range want {
		if got, _ := json.Marshal(args[k]); string(got) != mustJSON(v) {
			t.Errorf("%s = %s, want %s", k, got, mustJSON(v))
		}
	}
	if tables := p.Mutations[1].Tables(); len(tables) != 1 || tables[0] != "a" {
		t.Fatalf("update tables = %v", tables)
	}
}

func TestUsableFiltersByKindRowsAndWeight(t *testing.T) {
	p, err := Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	none := func(string) int { return 0 }
	some := func(string) int { return 1 }
	names := func(ms []*Mutation) string {
		var out []string
		for _, m := range ms {
			out = append(out, m.Name+"/"+string(m.Kind))
		}
		return strings.Join(out, ",")
	}
	if got := names(p.Usable(none, "")); got != "a.create/create,a.bad/reject" {
		t.Errorf("without rows: %s", got)
	}
	if got := names(p.Usable(some, "")); got != "a.create/create,a.update/update,a.bad/reject" {
		t.Errorf("with rows: %s", got)
	}
	if got := names(p.Usable(some, Reject)); got != "a.bad/reject" {
		t.Errorf("rejects only: %s", got)
	}
}

func TestParseRefusesBadProfiles(t *testing.T) {
	tests := map[string]string{
		"empty":              `{"mutations": []}`,
		"unknown field":      `{"mutations": [], "extra": 1}`,
		"no reject":          `{"mutations": [{"name": "a", "kind": "create"}]}`,
		"only row mutations": `{"mutations": [{"name": "a", "kind": "update", "args": {"id": "{{row:a}}"}}, {"name": "b", "kind": "reject"}]}`,
		"unknown kind":       `{"mutations": [{"name": "a", "kind": "explode"}]}`,
		"missing kind":       `{"mutations": [{"name": "a"}, {"name": "b", "kind": "reject"}]}`,
		"no name":            `{"mutations": [{"kind": "create"}, {"name": "b", "kind": "reject"}]}`,
		"args not object":    `{"mutations": [{"name": "a", "kind": "create", "args": [1]}, {"name": "b", "kind": "reject"}]}`,
		"unknown holder":     `{"mutations": [{"name": "a", "kind": "create", "args": {"x": "{{uuid}}"}}, {"name": "b", "kind": "reject"}]}`,
		"row without table":  `{"mutations": [{"name": "a", "kind": "create", "args": {"x": "{{row}}"}}, {"name": "b", "kind": "reject"}]}`,
		"bad new_id":         `{"mutations": [{"name": "a", "kind": "create", "args": {"x": "{{new_id:x}}"}}, {"name": "b", "kind": "reject"}]}`,
		"negative weight":    `{"mutations": [{"name": "a", "kind": "create", "weight": -1}, {"name": "b", "kind": "reject"}]}`,
		"reject creates":     `{"mutations": [{"name": "a", "kind": "create"}, {"name": "b", "kind": "reject", "creates": ["a"]}]}`,
		"mutation field":     `{"mutations": [{"name": "a", "kind": "create", "nope": 1}, {"name": "b", "kind": "reject"}]}`,
	}
	for name, raw := range tests {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Errorf("%s: accepted %s", name, raw)
		}
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestLoadReadsAFile(t *testing.T) {
	path := t.TempDir() + "/profile.json"
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path + ".missing"); err == nil {
		t.Fatal("a missing file loaded")
	}
}
