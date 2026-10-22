package profile

import (
	"encoding/json"
	"fmt"
)

// Kind is what the suite may expect of a mutation.
type Kind string

// The kinds of mutation a profile declares.
const (
	// Create makes rows.
	Create Kind = "create"
	// Update changes existing rows. It may be rejected when the row is gone.
	Update Kind = "update"
	// Delete removes rows. It may be rejected when the row is gone.
	Delete Kind = "delete"
	// Reject must always be rejected by a business rule.
	Reject Kind = "reject"
)

// UnmarshalJSON accepts only the declared kinds.
func (k *Kind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch Kind(s) {
	case Create, Update, Delete, Reject:
		*k = Kind(s)
		return nil
	default:
		return fmt.Errorf("unknown kind %q (want create, update, delete or reject)", s)
	}
}
