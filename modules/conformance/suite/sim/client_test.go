package sim

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	storev1 "github.com/skkil/store/modules/conformance/gen/storev1"
	"github.com/skkil/store/modules/conformance/gen/storev1/storev1connect"
	"github.com/skkil/store/modules/conformance/suite/check"
	"github.com/skkil/store/modules/conformance/suite/credentials"
	"github.com/skkil/store/modules/conformance/suite/hashchain"
	"github.com/skkil/store/modules/conformance/suite/profile"
)

type fakeStore struct {
	storev1connect.UnimplementedStoreServiceHandler
	log         []*storev1.Version
	skip        uint64
	badHash     uint64
	neverReset  bool
	tamperFirst bool
}

func newFakeStore(n int) *fakeStore {
	f := &fakeStore{}
	var prev []byte
	client := uuid.NewString()
	for i := range n {
		v := &storev1.Version{Version: uint64(i + 1), ClientId: client, MutationId: uint64(i + 1), Changes: []*storev1.Change{
			{Table: "todo", RowId: uuid.NewString(), Op: storev1.Change_OP_UPSERT, Data: []byte(`{"title": "t"}`)},
		}}
		v.Hash = hashchain.Next(prev, v)
		prev = v.Hash
		f.log = append(f.log, v)
	}
	return f
}

func (f *fakeStore) Pull(_ context.Context, req *connect.Request[storev1.PullRequest]) (*connect.Response[storev1.PullResponse], error) {
	cursor := req.Msg.GetCursor()
	head := uint64(len(f.log))
	resp := &storev1.PullResponse{HeadVersion: head, Epoch: 1}
	stale := cursor.GetVersion() > head || (cursor.GetVersion() > 0 && string(cursor.GetHash()) != string(f.log[cursor.GetVersion()-1].GetHash()))
	if stale && !f.neverReset {
		resp.Body = &storev1.PullResponse_Reset_{Reset_: &storev1.Reset{Epoch: 1, Reason: "stale"}}
		return connect.NewResponse(resp), nil
	}
	var versions []*storev1.Version
	for _, v := range f.log[min(cursor.GetVersion(), head):] {
		if v.GetVersion() == f.skip {
			continue
		}
		out := &storev1.Version{Version: v.Version, ClientId: v.ClientId, MutationId: v.MutationId, Hash: v.Hash, Changes: v.Changes}
		if v.GetVersion() == f.badHash {
			out.Hash = []byte("forged")
		}
		if f.tamperFirst {
			out.Changes = []*storev1.Change{{Table: "todo", RowId: "evil", Op: storev1.Change_OP_UPSERT, Data: []byte(`{}`)}}
			f.tamperFirst = false
		}
		versions = append(versions, out)
	}
	resp.Body = &storev1.PullResponse_Entries{Entries: &storev1.Entries{Versions: versions}}
	return connect.NewResponse(resp), nil
}

type staticToken struct{}

func (staticToken) Fetch(_ context.Context, _, client string) (credentials.Credentials, error) {
	return credentials.Credentials{Token: "t", Scope: "s", Expiry: time.Now().Add(time.Hour)}, nil
}

func newTestClient(t *testing.T, f *fakeStore, ledger *check.Ledger, name string) *Client {
	t.Helper()
	_, handler := storev1connect.NewStoreServiceHandler(f)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	p, err := profile.Parse([]byte(`{"mutations":[{"name":"a","kind":"create"},{"name":"b","kind":"reject"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	c := New(Options{Name: name, User: "u", Profile: p, Transport: NewTransport(http.DefaultClient, server.URL, server.URL), Credentials: staticToken{}, Ledger: ledger, KeepLog: true})
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	return c
}

func invariants(l *check.Ledger) map[check.Invariant]bool {
	out := map[check.Invariant]bool{}
	for _, v := range l.Violations() {
		out[v.Invariant] = true
	}
	return out
}

func TestClientAcceptsAnHonestLog(t *testing.T) {
	ledger := check.NewLedger()
	c := newTestClient(t, newFakeStore(5), ledger, "c0")
	if _, err := c.PullAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	c.Corrupt(WrongHash)
	if _, err := c.PullAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if v := ledger.Violations(); len(v) != 0 || c.Resets() != 1 || c.Cursor().GetVersion() != 5 || len(c.Log()) != 5 {
		t.Fatalf("violations %v, resets %d, cursor %v", v, c.Resets(), c.Cursor())
	}
}

func TestClientCatchesAGap(t *testing.T) {
	f := newFakeStore(4)
	f.skip = 2
	ledger := check.NewLedger()
	if _, err := newTestClient(t, f, ledger, "c0").PullAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !invariants(ledger)[check.Dense] {
		t.Fatalf("violations: %v", ledger.Violations())
	}
}

func TestClientCatchesAForgedHash(t *testing.T) {
	f := newFakeStore(3)
	f.badHash = 2
	ledger := check.NewLedger()
	if _, err := newTestClient(t, f, ledger, "c0").PullAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !invariants(ledger)[check.HashChain] {
		t.Fatalf("violations: %v", ledger.Violations())
	}
}

func TestClientCatchesAMissingReset(t *testing.T) {
	f := newFakeStore(3)
	f.neverReset = true
	ledger := check.NewLedger()
	c := newTestClient(t, f, ledger, "c0")
	if _, err := c.PullAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	c.Corrupt(AheadOfHead)
	if _, err := c.PullAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !invariants(ledger)[check.ResetOnMismatch] {
		t.Fatalf("violations: %v", ledger.Violations())
	}
}

func TestClientCatchesAnUnexpectedReset(t *testing.T) {
	ledger := check.NewLedger()
	c := newTestClient(t, newFakeStore(3), ledger, "c0")
	if _, err := c.PullAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	c.cursor = &storev1.Cursor{Epoch: 1, Version: 2, Hash: []byte("wrong")}
	if _, err := c.PullAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !invariants(ledger)[check.ResetOnMismatch] {
		t.Fatalf("violations: %v", ledger.Violations())
	}
}

func TestClientsCatchAVersionDeliveredDifferently(t *testing.T) {
	f := newFakeStore(2)
	ledger := check.NewLedger()
	honest := newTestClient(t, f, ledger, "c0")
	if _, err := honest.PullAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.tamperFirst = true
	if _, err := newTestClient(t, f, ledger, "c1").PullAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := invariants(ledger); !got[check.Atomic] || !got[check.HashChain] {
		t.Fatalf("violations: %v", ledger.Violations())
	}
}

func TestOfflineClientDoesNothing(t *testing.T) {
	ledger := check.NewLedger()
	c := newTestClient(t, newFakeStore(1), ledger, "c0")
	c.Disconnect()
	c.Mutate(1, "")
	for _, op := range []func() (string, error){
		func() (string, error) { return c.Push(context.Background(), false) },
		func() (string, error) { return c.Pull(context.Background(), 1) },
		func() (string, error) { return c.PullAll(context.Background()) },
	} {
		if summary, err := op(); err != nil || summary != "offline" {
			t.Fatalf("offline step = %q, %v", summary, err)
		}
	}
	if c.PendingCount() != 1 || c.Cursor().GetVersion() != 0 {
		t.Fatalf("pending %d, cursor %v", c.PendingCount(), c.Cursor())
	}
}
