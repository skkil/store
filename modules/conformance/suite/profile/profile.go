// Package profile reads a mutation profile: the JSON description of a
// product's mutations that lets the conformance suite drive any product.
package profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Profile describes the mutations of one product.
type Profile struct {
	// SchemaVersion is sent as PushRequest.schema_version.
	SchemaVersion uint32     `json:"schema_version"`
	Mutations     []Mutation `json:"mutations"`
}

// Load reads and validates the profile at path.
func Load(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse reads and validates a profile.
func Parse(data []byte) (*Profile, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var p Profile
	if err := decoder.Decode(&p); err != nil {
		return nil, fmt.Errorf("profile: %w", err)
	}
	if err := p.validate(); err != nil {
		return nil, fmt.Errorf("profile: %w", err)
	}
	return &p, nil
}

func (p *Profile) validate() error {
	if len(p.Mutations) == 0 {
		return errors.New("no mutations")
	}
	var seedable, rejecting bool
	for i := range p.Mutations {
		m := &p.Mutations[i]
		if err := m.compile(); err != nil {
			return fmt.Errorf("mutations[%d] (%s): %w", i, m.Name, err)
		}
		if m.Kind == Reject {
			rejecting = true
		} else if len(m.Tables()) == 0 {
			seedable = true
		}
	}
	if !seedable {
		return errors.New("at least one create, update or delete mutation must work without {{row:…}}")
	}
	if !rejecting {
		return errors.New("at least one mutation must have kind \"reject\"")
	}
	return nil
}

// Usable returns the mutations of kind (any kind when "") whose row
// references can be filled from rows, a count of known rows per table.
func (p *Profile) Usable(rows func(table string) int, kind Kind) []*Mutation {
	var usable []*Mutation
	for i := range p.Mutations {
		m := &p.Mutations[i]
		if m.Weight == 0 || (kind != "" && m.Kind != kind) {
			continue
		}
		ok := true
		for _, table := range m.Tables() {
			if rows(table) == 0 {
				ok = false
				break
			}
		}
		if ok {
			usable = append(usable, m)
		}
	}
	return usable
}
