// Package check records what simulated clients send and receive and checks
// the store invariants (ARCHITECTURE.md §17) against it.
package check

// Invariant names one property the suite checks.
type Invariant string

// The invariants the suite checks.
const (
	// Dense: versions are dense and gap-free per scope, and monotonic for
	// each client's pulls and acks.
	Dense Invariant = "L1"
	// AtMostOnce: each (client, mutation) is applied at most once; a
	// re-push answers DUPLICATE and adds no version.
	AtMostOnce Invariant = "L2"
	// Atomic: no part of a mutation is visible alone; every delivery of a
	// version is whole and identical, and a rejection carries no changes.
	Atomic Invariant = "L3"
	// Identical: after convergence every replica equals every other.
	Identical Invariant = "L4"
	// DurableRejection: a rejected mutation is a rejection entry in the log
	// that every client pulls, and a mutation that must be rejected is.
	DurableRejection Invariant = "S2"
	// HashChain: every version's hash matches the chain recomputed from the
	// pulled versions.
	HashChain Invariant = "HASH"
	// ResetOnMismatch: a cursor whose epoch, hash or position no longer
	// matches the log gets Reset, and only such a cursor does.
	ResetOnMismatch Invariant = "RESET"
	// DerivedIDs: an applied mutation upserts the rows it creates under the
	// ids derived from it (docs/protocol.md, Ids).
	DerivedIDs Invariant = "ID"
	// PokesReachHead: a client holding a Pokes stream is poked up to the head.
	PokesReachHead Invariant = "POKE"
	// Liveness: every pushed mutation is eventually acknowledged and every
	// client reaches the head.
	Liveness Invariant = "LIVE"
	// WellFormed: responses follow the protocol's shape (one result per
	// mutation, statuses set, versions where required).
	WellFormed Invariant = "PROTO"
)
