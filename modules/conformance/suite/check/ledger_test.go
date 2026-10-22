package check

import (
	"testing"

	storev1 "github.com/skkil/store/modules/conformance/gen/storev1"
	"github.com/skkil/store/modules/conformance/suite/profile"
)

const client = "11111111-1111-1111-1111-111111111111"

func result(id uint64, status storev1.MutationResult_Status, version uint64) *storev1.MutationResult {
	return &storev1.MutationResult{MutationId: id, Status: status, Version: version}
}

func broke(l *Ledger) map[Invariant]int {
	out := map[Invariant]int{}
	for _, v := range l.Violations() {
		out[v.Invariant]++
	}
	return out
}

const (
	applied   = storev1.MutationResult_STATUS_APPLIED
	rejected  = storev1.MutationResult_STATUS_REJECTED
	duplicate = storev1.MutationResult_STATUS_DUPLICATE
)

func TestPushedAcceptsDuplicatesAfterAnOutcome(t *testing.T) {
	l := NewLedger()
	l.Pushed(client, []uint64{1, 2}, []*storev1.MutationResult{result(1, applied, 1), result(2, rejected, 2)})
	l.Pushed(client, []uint64{1, 2}, []*storev1.MutationResult{result(1, duplicate, 1), result(2, duplicate, 0)})
	l.Pushed(client, []uint64{1}, []*storev1.MutationResult{result(1, duplicate, 7)})
	if v := l.Violations(); len(v) != 0 {
		t.Fatalf("violations: %v", v)
	}
}

func TestPushedCatchesASecondApplication(t *testing.T) {
	l := NewLedger()
	l.Pushed(client, []uint64{1}, []*storev1.MutationResult{result(1, applied, 5)})
	l.Pushed(client, []uint64{1}, []*storev1.MutationResult{result(1, applied, 6)})
	l.Pushed(client, []uint64{1}, []*storev1.MutationResult{result(1, duplicate, 9)})
	l.Pushed(client, []uint64{1}, []*storev1.MutationResult{result(1, duplicate, 2)})
	if got := broke(l)[AtMostOnce]; got != 2 {
		t.Fatalf("L2 violations = %d, want 2: %v", got, l.Violations())
	}
}

func TestPushedCatchesMalformedResponses(t *testing.T) {
	l := NewLedger()
	l.Pushed(client, []uint64{1, 2}, []*storev1.MutationResult{result(2, applied, 1)})
	l.Pushed(client, []uint64{3}, []*storev1.MutationResult{result(3, applied, 0)})
	l.Pushed(client, []uint64{4}, []*storev1.MutationResult{result(4, storev1.MutationResult_STATUS_UNSPECIFIED, 4)})
	if got := broke(l)[WellFormed]; got != 4 {
		t.Fatalf("PROTO violations = %d, want 4: %v", got, l.Violations())
	}
}

func TestPushedCatchesAnAppliedRejection(t *testing.T) {
	l := NewLedger()
	l.Expect(MutationKey{client, 1}, Expectation{Name: "todo.create", Kind: profile.Reject})
	l.Pushed(client, []uint64{1}, []*storev1.MutationResult{result(1, applied, 1)})
	if broke(l)[DurableRejection] != 1 {
		t.Fatalf("violations: %v", l.Violations())
	}
}

func TestDeliveredCatchesDifferentDeliveriesAndDirtyRejections(t *testing.T) {
	l := NewLedger()
	v := &storev1.Version{Version: 1, ClientId: client, MutationId: 1, Changes: []*storev1.Change{{Table: "t", RowId: "a"}}}
	l.Delivered(client, 1, v)
	l.Delivered("other", 1, v)
	l.Delivered("other", 2, &storev1.Version{Version: 1})
	if len(l.Violations()) != 0 {
		t.Fatalf("identical deliveries flagged: %v", l.Violations())
	}
	l.Delivered("other", 1, &storev1.Version{Version: 1, ClientId: client, MutationId: 1})
	l.Delivered(client, 1, &storev1.Version{Version: 2, MutationId: 2, Rejection: &storev1.Rejection{MutationId: 2}, Changes: []*storev1.Change{{}}})
	l.Delivered(client, 1, &storev1.Version{Version: 3, MutationId: 3, Rejection: &storev1.Rejection{MutationId: 4}})
	got := broke(l)
	if got[Atomic] != 2 || got[WellFormed] != 1 {
		t.Fatalf("violations: %v", l.Violations())
	}
}

func TestViolationsAreDeduplicatedAndCapped(t *testing.T) {
	l := NewLedger()
	for range 3 {
		l.Violate(Dense, "c0", "same")
	}
	if len(l.Violations()) != 1 {
		t.Fatalf("violations: %v", l.Violations())
	}
	for i := range MaxViolations + 5 {
		l.Violate(Dense, "c0", "v%d", i)
	}
	all := l.Violations()
	if len(all) != MaxViolations+1 || all[MaxViolations].Invariant != "…" {
		t.Fatalf("kept %d violations, last %v", len(all), all[len(all)-1])
	}
}

func commit(version, mutation uint64, changes ...*storev1.Change) *storev1.Version {
	return &storev1.Version{Version: version, ClientId: client, MutationId: mutation, Changes: changes}
}

func rejection(version, mutation uint64, reason string) *storev1.Version {
	return &storev1.Version{Version: version, ClientId: client, MutationId: mutation, Rejection: &storev1.Rejection{MutationId: mutation, Reason: reason}}
}

func TestAuditPassesAConsistentRun(t *testing.T) {
	l := NewLedger()
	l.Expect(MutationKey{client, 1}, Expectation{Kind: profile.Create, Created: []CreatedRow{{Table: "t", ID: "a"}}})
	l.Expect(MutationKey{client, 2}, Expectation{Kind: profile.Reject})
	l.Pushed(client, []uint64{1, 2}, []*storev1.MutationResult{result(1, applied, 1), {MutationId: 2, Status: rejected, Version: 3, Reason: "no"}})
	state := State{"t": {"a": `{"x":1}`}}
	head := &storev1.Cursor{Epoch: 1, Version: 3, Hash: []byte{3}}
	l.Audit(Converged{
		Log: []*storev1.Version{
			commit(1, 1, &storev1.Change{Table: "t", RowId: "a", Op: storev1.Change_OP_UPSERT}),
			{Version: 2, ClientId: ServerClientID, MutationId: 2},
			rejection(3, 2, "no"),
		},
		Head:    head,
		State:   state,
		Clients: []ClientState{{Name: "c0", Cursor: head, State: State{"t": {"a": `{"x":1}`}}}},
	})
	if v := l.Violations(); len(v) != 0 {
		t.Fatalf("violations: %v", v)
	}
}

func TestAuditCatchesEachBrokenInvariant(t *testing.T) {
	l := NewLedger()
	l.Expect(MutationKey{client, 1}, Expectation{Kind: profile.Create, Created: []CreatedRow{{Table: "t", ID: "a"}}})
	l.Expect(MutationKey{client, 4}, Expectation{Kind: profile.Reject})
	l.Pushed(client, []uint64{1, 2, 3, 5}, []*storev1.MutationResult{
		result(1, applied, 1),
		{MutationId: 2, Status: rejected, Version: 9, Reason: "gone"},
		{MutationId: 3, Status: rejected, Version: 3, Reason: "a"},
		result(5, applied, 7),
	})
	head := &storev1.Cursor{Epoch: 1, Version: 5, Hash: []byte{5}}
	l.Audit(Converged{
		Log: []*storev1.Version{
			commit(1, 1),
			rejection(3, 3, "b"),
			commit(3, 4),
			commit(4, 1),
		},
		Head:  head,
		State: State{"t": {"a": "1"}},
		Clients: []ClientState{
			{Name: "c0", Pending: 1, Cursor: head, State: State{"t": {"a": "2"}}},
			{Name: "c1", Cursor: &storev1.Cursor{Epoch: 1, Version: 4}},
		},
	})
	got := broke(l)
	for _, inv := range []Invariant{Dense, AtMostOnce, DurableRejection, DerivedIDs, Liveness, Identical} {
		if got[inv] == 0 {
			t.Errorf("%s not caught; violations: %v", inv, l.Violations())
		}
	}
}

func TestStateDiff(t *testing.T) {
	a := State{"t": {"1": "x", "2": "y"}}
	if d := a.Diff(State{"t": {"1": "x", "2": "y"}}); d != "" {
		t.Fatalf("equal states differ: %s", d)
	}
	for _, other := range []State{{"t": {"1": "x"}}, {"t": {"1": "x", "2": "z"}}, {"t": {"1": "x", "2": "y", "3": "w"}}, {"u": {"1": "x"}}} {
		if a.Diff(other) == "" {
			t.Errorf("%v and %v reported equal", a, other)
		}
	}
	if a.Rows() != 2 {
		t.Fatalf("rows = %d", a.Rows())
	}
}
