package writepath

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Mutation is what a mutation body sees: the open transaction and the
// identity of the device mutation it runs.
type Mutation struct {
	// Tx is the transaction that holds the scope lock. Every row change of
	// the mutation goes through it.
	Tx     pgx.Tx
	Scope  uuid.UUID
	Client uuid.UUID
	ID     uint64
	ids    uint32
}

// NextID returns the next derived row id of this mutation, the same id the
// device created optimistically.
func (m *Mutation) NextID() uuid.UUID {
	id := DeriveID(m.Client, m.ID, m.ids)
	m.ids++
	return id
}
