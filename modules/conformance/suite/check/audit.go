package check

import (
	"bytes"
	"cmp"
	"maps"
	"slices"

	storev1 "github.com/skkil/store/modules/conformance/gen/storev1"
	"github.com/skkil/store/modules/conformance/suite/profile"
)

// ServerClientID is the client id of writes a product makes on its own.
const ServerClientID = "00000000-0000-0000-0000-000000000000"

// ClientState is one simulated client after convergence.
type ClientState struct {
	Name    string
	Pending int
	Cursor  *storev1.Cursor
	State   State
}

// Converged is the scope after every client tried to converge: the whole
// log as a fresh auditor pulled it, its replica, and every client's state.
type Converged struct {
	Log     []*storev1.Version
	Head    *storev1.Cursor
	State   State
	Clients []ClientState
}

// Audit runs the checks that need the whole log: density, at-most-once and
// per-client order over the log, durable rejections, derived ids, liveness
// and replica equality.
func (l *Ledger) Audit(c Converged) {
	l.mu.Lock()
	defer l.mu.Unlock()
	byKey := l.auditLog(c.Log)
	l.auditOutcomes(byKey)
	l.auditExpectations(byKey)
	for _, client := range c.Clients {
		if client.Pending > 0 {
			l.violate(Liveness, client.Name, "%d mutations still pending after convergence", client.Pending)
		}
		if client.Cursor.GetVersion() != c.Head.GetVersion() || !bytes.Equal(client.Cursor.GetHash(), c.Head.GetHash()) {
			l.violate(Liveness, client.Name, "cursor at v%d, head is v%d", client.Cursor.GetVersion(), c.Head.GetVersion())
			continue
		}
		if diff := client.State.Diff(c.State); diff != "" {
			l.violate(Identical, client.Name, "replica differs from a fresh pull of the log: %s", diff)
		}
	}
}

func (l *Ledger) auditLog(log []*storev1.Version) map[MutationKey]*storev1.Version {
	byKey := map[MutationKey]*storev1.Version{}
	lastByClient := map[string]uint64{}
	for i, v := range log {
		if v.GetVersion() != uint64(i+1) {
			l.violate(Dense, "", "log position %d holds v%d", i+1, v.GetVersion())
		}
		if v.GetClientId() == ServerClientID {
			continue
		}
		name := l.nameOf(v.GetClientId())
		key := MutationKey{Client: v.GetClientId(), MutationID: v.GetMutationId()}
		if first, ok := byKey[key]; ok {
			l.violate(AtMostOnce, name, "m%d recorded twice: v%d and v%d", key.MutationID, first.GetVersion(), v.GetVersion())
			continue
		}
		byKey[key] = v
		if last := lastByClient[key.Client]; key.MutationID <= last {
			l.violate(Dense, name, "m%d recorded at v%d after m%d", key.MutationID, v.GetVersion(), last)
		}
		lastByClient[key.Client] = key.MutationID
	}
	return byKey
}

func (l *Ledger) auditOutcomes(byKey map[MutationKey]*storev1.Version) {
	for _, key := range sortedKeys(l.outcomes) {
		outcome := l.outcomes[key]
		name := l.nameOf(key.Client)
		v, ok := byKey[key]
		rejected := outcome.Status == storev1.MutationResult_STATUS_REJECTED
		switch {
		case !ok && rejected:
			l.violate(DurableRejection, name, "m%d was answered REJECTED but the log has no entry for it", key.MutationID)
		case !ok:
			l.violate(AtMostOnce, name, "m%d was answered APPLIED v%d but the log has no entry for it", key.MutationID, outcome.Version)
		case v.GetVersion() != outcome.Version:
			l.violate(AtMostOnce, name, "m%d was answered v%d but the log records it at v%d", key.MutationID, outcome.Version, v.GetVersion())
		case rejected && v.GetRejection() == nil:
			l.violate(DurableRejection, name, "m%d was answered REJECTED but v%d is a commit", key.MutationID, v.GetVersion())
		case rejected && v.GetRejection().GetReason() != outcome.Reason:
			l.violate(DurableRejection, name, "m%d rejected for %q, the log says %q", key.MutationID, outcome.Reason, v.GetRejection().GetReason())
		case !rejected && v.GetRejection() != nil:
			l.violate(AtMostOnce, name, "m%d was answered APPLIED but v%d is a rejection", key.MutationID, v.GetVersion())
		}
	}
}

func (l *Ledger) auditExpectations(byKey map[MutationKey]*storev1.Version) {
	for _, key := range sortedKeys(l.expected) {
		expected := l.expected[key]
		v, ok := byKey[key]
		if !ok {
			continue
		}
		name := l.nameOf(key.Client)
		if v.GetRejection() != nil {
			continue
		}
		if expected.Kind == profile.Reject {
			l.violate(DurableRejection, name, "m%d (%s) must be rejected but v%d commits it", key.MutationID, expected.Name, v.GetVersion())
			continue
		}
		for _, row := range expected.Created {
			if !upserts(v, row) {
				l.violate(DerivedIDs, name, "m%d (%s) applied at v%d without upserting %s/%s", key.MutationID, expected.Name, v.GetVersion(), row.Table, row.ID)
			}
		}
	}
}

func upserts(v *storev1.Version, row CreatedRow) bool {
	for _, c := range v.GetChanges() {
		if c.GetOp() == storev1.Change_OP_UPSERT && c.GetTable() == row.Table && c.GetRowId() == row.ID {
			return true
		}
	}
	return false
}

func sortedKeys[V any](m map[MutationKey]V) []MutationKey {
	return slices.SortedFunc(maps.Keys(m), func(a, b MutationKey) int {
		if c := cmp.Compare(a.Client, b.Client); c != 0 {
			return c
		}
		return cmp.Compare(a.MutationID, b.MutationID)
	})
}
