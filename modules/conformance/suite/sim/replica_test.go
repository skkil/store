package sim

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	storev1 "github.com/skkil/store/modules/conformance/gen/storev1"
)

func upsert(table, id, data string) *storev1.Change {
	return &storev1.Change{Table: table, RowId: id, Op: storev1.Change_OP_UPSERT, Data: []byte(data)}
}

func TestReplicaAppliesUpsertsAndDeletes(t *testing.T) {
	r := NewReplica()
	if err := r.Apply(&storev1.Version{Version: 1, Changes: []*storev1.Change{
		upsert("todo", "b", `{"title": "B", "n": 1.50}`),
		upsert("todo", "a", `{"n": 2, "title": "A"}`),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Apply(&storev1.Version{Version: 2, Changes: []*storev1.Change{
		upsert("todo", "b", `{"title": "B2", "n": 1.50}`),
		{Table: "todo", RowId: "a", Op: storev1.Change_OP_DELETE},
		upsert("todo", "c", `{}`),
	}}); err != nil {
		t.Fatal(err)
	}
	state := r.State()
	if state["todo"]["b"] != `{"n":1.50,"title":"B2"}` {
		t.Fatalf("b = %s", state["todo"]["b"])
	}
	if _, ok := state["todo"]["a"]; ok || r.Count("todo") != 2 {
		t.Fatalf("state = %v", state)
	}
	if ids := r.IDs("todo"); len(ids) != 2 || ids[0] != "b" || ids[1] != "c" {
		t.Fatalf("ids in arrival order = %v", ids)
	}
	r.Clear()
	if r.Count("todo") != 0 || len(r.State()) != 0 {
		t.Fatal("Clear left rows")
	}
}

func TestReplicaRefusesRowsThatAreNotObjects(t *testing.T) {
	r := NewReplica()
	if err := r.Apply(&storev1.Version{Version: 1, Changes: []*storev1.Change{upsert("todo", "a", `[1]`)}}); err == nil {
		t.Fatal("a JSON array was accepted as a row")
	}
}

func TestRetryable(t *testing.T) {
	for err, want := range map[error]bool{
		errors.New("connection refused"):                      true,
		connect.NewError(connect.CodeUnavailable, nil):        true,
		connect.NewError(connect.CodeInternal, nil):           true,
		connect.NewError(connect.CodeUnauthenticated, nil):    false,
		connect.NewError(connect.CodePermissionDenied, nil):   false,
		connect.NewError(connect.CodeFailedPrecondition, nil): false,
	} {
		if got := Retryable(err); got != want {
			t.Errorf("Retryable(%v) = %v, want %v", err, got, want)
		}
	}
}
