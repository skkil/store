package profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Mutation is one product mutation as a JSON template.
//
// Every string in Args may hold placeholders, filled per pushed mutation:
//
//	{{new_id}}, {{new_id:N}}  the N-th id (from 0) derived from the mutation (docs/protocol.md, Ids)
//	{{row:TABLE}}             the id of a row of TABLE the client has pulled
//	{{text}}                  a few random words
//	{{int}}                   a random integer (a JSON number when it is the whole string)
//	{{bool}}                  a random boolean (a JSON boolean when it is the whole string)
//	{{client_id}}             the client id
//	{{mutation_id}}           the mutation id (a JSON number when it is the whole string)
type Mutation struct {
	Name string `json:"name"`
	Kind Kind   `json:"kind"`
	// Weight is how often the mutation is picked relative to the others; 0
	// leaves it out. Missing means 1.
	Weight int             `json:"weight"`
	Args   json.RawMessage `json:"args"`
	// Creates lists, in order, the table of each row the mutation creates
	// with a derived id: the n-th entry gets the n-th derived id. When the
	// mutation applies, its version must upsert each of those rows.
	Creates []string `json:"creates"`

	template any
	tables   []string
}

// Env fills placeholders for one pushed mutation.
type Env interface {
	NewID(n uint32) string
	Row(table string) string
	Text() string
	Int() int64
	Bool() bool
	ClientID() string
	MutationID() uint64
}

var placeholder = regexp.MustCompile(`\{\{([a-z_]+)(?::([^}]+))?\}\}`)

// UnmarshalJSON defaults Weight to 1.
func (m *Mutation) UnmarshalJSON(data []byte) error {
	type plain Mutation
	p := plain{Weight: 1}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return err
	}
	*m = Mutation(p)
	return nil
}

// Tables returns the tables named by {{row:TABLE}} placeholders.
func (m *Mutation) Tables() []string { return m.tables }

func (m *Mutation) compile() error {
	if m.Name == "" {
		return errors.New("name is empty")
	}
	if m.Kind == "" {
		return errors.New("kind is missing")
	}
	if m.Weight < 0 {
		return errors.New("weight is negative")
	}
	if m.Kind == Reject && len(m.Creates) > 0 {
		return errors.New("a reject mutation creates nothing")
	}
	if len(m.Args) == 0 {
		m.Args = json.RawMessage("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(m.Args))
	decoder.UseNumber()
	if err := decoder.Decode(&m.template); err != nil {
		return fmt.Errorf("args: %w", err)
	}
	if _, ok := m.template.(map[string]any); !ok {
		return errors.New("args must be a JSON object")
	}
	tables := map[string]bool{}
	var problems []string
	walkStrings(m.template, func(s string) {
		for _, match := range placeholder.FindAllStringSubmatch(s, -1) {
			name, arg := match[1], match[2]
			switch name {
			case "row":
				if arg == "" {
					problems = append(problems, "{{row}} needs a table: {{row:TABLE}}")
				}
				tables[arg] = true
			case "new_id":
				if arg == "" {
					break
				}
				if _, err := strconv.ParseUint(arg, 10, 32); err != nil {
					problems = append(problems, fmt.Sprintf("{{new_id:%s}} needs a number", arg))
				}
			case "text", "int", "bool", "client_id", "mutation_id":
			default:
				problems = append(problems, fmt.Sprintf("unknown placeholder {{%s}}", name))
			}
		}
	})
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	for table := range tables {
		if table != "" {
			m.tables = append(m.tables, table)
		}
	}
	slices.Sort(m.tables)
	return nil
}

// Render fills the template's placeholders and returns the JSON arguments.
// Object keys are visited in sorted order, so one Env draws its random
// values in the same order every time.
func (m *Mutation) Render(env Env) ([]byte, error) {
	return json.Marshal(fill(m.template, env))
}

func fill(value any, env Env) any {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(v))
		for _, k := range keys {
			out[k] = fill(v[k], env)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = fill(item, env)
		}
		return out
	case string:
		return fillString(v, env)
	default:
		return v
	}
}

func fillString(s string, env Env) any {
	if match := placeholder.FindStringSubmatch(s); match != nil && match[0] == s {
		switch match[1] {
		case "int":
			return env.Int()
		case "bool":
			return env.Bool()
		case "mutation_id":
			return env.MutationID()
		}
	}
	return placeholder.ReplaceAllStringFunc(s, func(token string) string {
		match := placeholder.FindStringSubmatch(token)
		switch match[1] {
		case "new_id":
			n, _ := strconv.ParseUint(match[2], 10, 32)
			return env.NewID(uint32(n))
		case "row":
			return env.Row(match[2])
		case "text":
			return env.Text()
		case "int":
			return strconv.FormatInt(env.Int(), 10)
		case "bool":
			return strconv.FormatBool(env.Bool())
		case "client_id":
			return env.ClientID()
		case "mutation_id":
			return strconv.FormatUint(env.MutationID(), 10)
		default:
			return token
		}
	})
}

func walkStrings(value any, visit func(string)) {
	switch v := value.(type) {
	case map[string]any:
		for _, item := range v {
			walkStrings(item, visit)
		}
	case []any:
		for _, item := range v {
			walkStrings(item, visit)
		}
	case string:
		visit(v)
	}
}
