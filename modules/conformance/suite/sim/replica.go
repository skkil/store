package sim

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	storev1 "github.com/skkil/store/modules/conformance/gen/storev1"
	"github.com/skkil/store/modules/conformance/suite/check"
)

type row struct {
	data  string
	order uint64
}

// Replica is a client's synced rows, applied from pulled versions.
type Replica struct {
	tables map[string]map[string]row
	order  uint64
}

// NewReplica returns an empty Replica.
func NewReplica() *Replica {
	return &Replica{tables: map[string]map[string]row{}}
}

// Apply applies every change of v. It returns an error for a change whose
// data is not a JSON object.
func (r *Replica) Apply(v *storev1.Version) error {
	for _, c := range v.GetChanges() {
		rows := r.tables[c.GetTable()]
		if rows == nil {
			rows = map[string]row{}
			r.tables[c.GetTable()] = rows
		}
		if c.GetOp() == storev1.Change_OP_DELETE {
			delete(rows, c.GetRowId())
			continue
		}
		canonical, err := canonicalJSON(c.GetData())
		if err != nil {
			return fmt.Errorf("v%d %s/%s: %w", v.GetVersion(), c.GetTable(), c.GetRowId(), err)
		}
		existing, ok := rows[c.GetRowId()]
		if !ok {
			r.order++
			existing.order = r.order
		}
		existing.data = canonical
		rows[c.GetRowId()] = existing
	}
	return nil
}

// Clear drops every row, as a client does on Reset.
func (r *Replica) Clear() {
	r.tables = map[string]map[string]row{}
	r.order = 0
}

// Count returns how many rows table holds.
func (r *Replica) Count(table string) int {
	return len(r.tables[table])
}

// IDs returns the row ids of table in the order the rows first arrived, so
// a seeded pick is reproducible whatever the ids are.
func (r *Replica) IDs(table string) []string {
	rows := r.tables[table]
	ids := slices.Collect(maps.Keys(rows))
	slices.SortFunc(ids, func(a, b string) int { return cmp.Compare(rows[a].order, rows[b].order) })
	return ids
}

// State returns the rows for comparison.
func (r *Replica) State() check.State {
	state := check.State{}
	for table, rows := range r.tables {
		if len(rows) == 0 {
			continue
		}
		out := make(map[string]string, len(rows))
		for id, row := range rows {
			out[id] = row.data
		}
		state[table] = out
	}
	return state
}

func canonicalJSON(data []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("row data is not a JSON object: %w", err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(canonical), nil
}
