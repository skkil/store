package check

import (
	"fmt"
	"maps"
	"slices"
)

// State is a replica's synced rows: table → row id → the row as canonical JSON.
type State map[string]map[string]string

// Diff describes the first difference between s and other, or "" when they are equal.
func (s State) Diff(other State) string {
	tables := slices.Sorted(maps.Keys(s))
	for table := range other {
		if _, ok := s[table]; !ok {
			tables = append(tables, table)
		}
	}
	slices.Sort(tables)
	for _, table := range slices.Compact(tables) {
		mine, theirs := s[table], other[table]
		ids := slices.Sorted(maps.Keys(mine))
		for id := range theirs {
			if _, ok := mine[id]; !ok {
				ids = append(ids, id)
			}
		}
		slices.Sort(ids)
		for _, id := range slices.Compact(ids) {
			a, inA := mine[id]
			b, inB := theirs[id]
			switch {
			case !inA:
				return fmt.Sprintf("%s/%s missing, other has %s", table, id, b)
			case !inB:
				return fmt.Sprintf("%s/%s is %s, other lacks it", table, id, a)
			case a != b:
				return fmt.Sprintf("%s/%s is %s, other has %s", table, id, a, b)
			}
		}
	}
	return ""
}

// Rows counts the rows in every table.
func (s State) Rows() int {
	n := 0
	for _, rows := range s {
		n += len(rows)
	}
	return n
}
