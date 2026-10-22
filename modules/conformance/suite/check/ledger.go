package check

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"

	storev1 "github.com/skkil/store/modules/conformance/gen/storev1"
	"github.com/skkil/store/modules/conformance/suite/profile"
)

// MaxViolations caps how many violations one scenario keeps.
const MaxViolations = 50

// MutationKey identifies a device mutation.
type MutationKey struct {
	Client     string
	MutationID uint64
}

// Expectation is what a simulated client made.
type Expectation struct {
	Name string
	Kind profile.Kind
	// Created are the derived row ids the mutation creates, by table.
	Created []CreatedRow
}

// CreatedRow is a row a mutation creates with a derived id.
type CreatedRow struct {
	Table string
	ID    string
}

// Outcome is the first APPLIED or REJECTED result a push returned.
type Outcome struct {
	Status  storev1.MutationResult_Status
	Version uint64
	Reason  string
}

type deliveryKey struct {
	epoch   uint32
	version uint64
}

// Ledger is shared by every client of one scenario. It is safe for
// concurrent use.
type Ledger struct {
	mu         sync.Mutex
	names      map[string]string
	expected   map[MutationKey]Expectation
	outcomes   map[MutationKey]Outcome
	deliveries map[deliveryKey][]byte
	violations []Violation
	dropped    int
}

// NewLedger returns an empty Ledger.
func NewLedger() *Ledger {
	return &Ledger{
		names:      map[string]string{},
		expected:   map[MutationKey]Expectation{},
		outcomes:   map[MutationKey]Outcome{},
		deliveries: map[deliveryKey][]byte{},
	}
}

// Name gives a client id a short name for messages.
func (l *Ledger) Name(clientID, name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.names[clientID] = name
}

// NameOf returns the short name of a client id, or the id itself.
func (l *Ledger) NameOf(clientID string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.nameOf(clientID)
}

func (l *Ledger) nameOf(clientID string) string {
	if name, ok := l.names[clientID]; ok {
		return name
	}
	return clientID
}

// Expect records a mutation a client made.
func (l *Ledger) Expect(key MutationKey, e Expectation) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.expected[key] = e
}

// Violate records a violation.
func (l *Ledger) Violate(invariant Invariant, client, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.violate(invariant, client, format, args...)
}

func (l *Ledger) violate(invariant Invariant, client, format string, args ...any) {
	v := Violation{Invariant: invariant, Client: client, Message: fmt.Sprintf(format, args...)}
	if slices.Contains(l.violations, v) {
		return
	}
	if len(l.violations) >= MaxViolations {
		l.dropped++
		return
	}
	l.violations = append(l.violations, v)
}

// Violations returns every recorded violation.
func (l *Ledger) Violations() []Violation {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := append([]Violation(nil), l.violations...)
	if l.dropped > 0 {
		out = append(out, Violation{Invariant: "…", Message: fmt.Sprintf("%d more violations not kept", l.dropped)})
	}
	return out
}

// Pushed checks one Push response: one result per sent mutation in order,
// at most one APPLIED or REJECTED per mutation over the whole run, and
// REJECTED for every mutation that must be rejected.
func (l *Ledger) Pushed(client string, sent []uint64, results []*storev1.MutationResult) {
	l.mu.Lock()
	defer l.mu.Unlock()
	name := l.nameOf(client)
	if len(results) != len(sent) {
		l.violate(WellFormed, name, "pushed %d mutations, got %d results", len(sent), len(results))
	}
	for i, r := range results {
		if i < len(sent) && r.GetMutationId() != sent[i] {
			l.violate(WellFormed, name, "result %d is for m%d, want m%d", i, r.GetMutationId(), sent[i])
			continue
		}
		l.pushedOne(name, MutationKey{Client: client, MutationID: r.GetMutationId()}, r)
	}
}

func (l *Ledger) pushedOne(name string, key MutationKey, r *storev1.MutationResult) {
	status := r.GetStatus()
	prior, seen := l.outcomes[key]
	switch status {
	case storev1.MutationResult_STATUS_APPLIED, storev1.MutationResult_STATUS_REJECTED:
		if r.GetVersion() == 0 {
			l.violate(WellFormed, name, "m%d %s without a version", key.MutationID, statusName(status))
		}
		if seen {
			l.violate(AtMostOnce, name, "m%d processed twice: %s v%d, then %s v%d", key.MutationID, statusName(prior.Status), prior.Version, statusName(status), r.GetVersion())
			return
		}
		l.outcomes[key] = Outcome{Status: status, Version: r.GetVersion(), Reason: r.GetReason()}
		if status == storev1.MutationResult_STATUS_APPLIED && l.expected[key].Kind == profile.Reject {
			l.violate(DurableRejection, name, "m%d (%s) must be rejected but was applied at v%d", key.MutationID, l.expected[key].Name, r.GetVersion())
		}
	case storev1.MutationResult_STATUS_DUPLICATE:
		if seen && r.GetVersion() != 0 && r.GetVersion() < prior.Version {
			l.violate(AtMostOnce, name, "m%d duplicate names v%d, before v%d that recorded it", key.MutationID, r.GetVersion(), prior.Version)
		}
	default:
		l.violate(WellFormed, name, "m%d has status %s", key.MutationID, statusName(status))
	}
}

// Delivered checks that every delivery of one version of one epoch is
// identical and that a rejection carries no changes.
func (l *Ledger) Delivered(client string, epoch uint32, v *storev1.Version) {
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(v)
	if err != nil {
		l.Violate(WellFormed, l.NameOf(client), "v%d does not encode: %v", v.GetVersion(), err)
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	name := l.nameOf(client)
	if v.GetRejection() != nil && len(v.GetChanges()) > 0 {
		l.violate(Atomic, name, "v%d is a rejection that carries %d changes", v.GetVersion(), len(v.GetChanges()))
	}
	if r := v.GetRejection(); r != nil && r.GetMutationId() != v.GetMutationId() {
		l.violate(WellFormed, name, "v%d rejection names m%d, version names m%d", v.GetVersion(), r.GetMutationId(), v.GetMutationId())
	}
	key := deliveryKey{epoch: epoch, version: v.GetVersion()}
	if first, ok := l.deliveries[key]; !ok {
		l.deliveries[key] = encoded
	} else if !bytes.Equal(first, encoded) {
		l.violate(Atomic, name, "epoch %d v%d differs from an earlier delivery of the same version", epoch, v.GetVersion())
	}
}

// Outcomes returns the first APPLIED or REJECTED result of every mutation.
func (l *Ledger) Outcomes() map[MutationKey]Outcome {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[MutationKey]Outcome, len(l.outcomes))
	for k, v := range l.outcomes {
		out[k] = v
	}
	return out
}

// Expectations returns every mutation the clients made.
func (l *Ledger) Expectations() map[MutationKey]Expectation {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[MutationKey]Expectation, len(l.expected))
	for k, v := range l.expected {
		out[k] = v
	}
	return out
}

func statusName(s storev1.MutationResult_Status) string {
	return strings.TrimPrefix(s.String(), "STATUS_")
}
