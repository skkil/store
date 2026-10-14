package pull_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	storev1 "github.com/skkil/store/modules/service/gen/storev1"
	"github.com/skkil/store/modules/service/internal/auth"
	"github.com/skkil/store/modules/service/internal/pull"
	"github.com/skkil/store/modules/service/internal/storetest"
)

type fixture struct {
	db      *storetest.Database
	writer  storetest.Writer
	service *pull.Service
	id      auth.Identity
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := storetest.NewDatabase(t)
	return &fixture{
		db:      db,
		writer:  storetest.Writer{Pool: db.Pool},
		service: pull.NewService(pull.NewLog(db.Pool), 300*time.Millisecond),
		id:      auth.Identity{Scope: uuid.NewString(), Client: uuid.NewString()},
	}
}

func (f *fixture) mutate(t *testing.T, id uint64, body func(pgx.Tx) error) storetest.Result {
	t.Helper()
	result, err := f.writer.Mutate(context.Background(), f.id.Scope, f.id.Client, id, body)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func (f *fixture) pull(t *testing.T, req pull.Request) *storev1.PullResponse {
	t.Helper()
	resp, err := f.service.Pull(context.Background(), f.id, req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (f *fixture) serverVersions(t *testing.T, n int) {
	t.Helper()
	_, err := f.db.Pool.Exec(context.Background(), `
		DO $$ BEGIN
		  FOR i IN 1..`+strconv.Itoa(n)+` LOOP
		    PERFORM store_begin_server('`+f.id.Scope+`');
		    PERFORM store_end();
		  END LOOP;
		END $$`)
	if err != nil {
		t.Fatal(err)
	}
}

func entries(t *testing.T, resp *storev1.PullResponse) *storev1.Entries {
	t.Helper()
	if resp.GetEntries() == nil {
		t.Fatalf("expected entries, got %v", resp)
	}
	return resp.GetEntries()
}

func cursorAt(v *storev1.Version, epoch uint32) pull.Cursor {
	return pull.Cursor{Epoch: epoch, Version: v.GetVersion(), Hash: v.GetHash()}
}

func TestPullReturnsVersionsInOrder(t *testing.T) {
	f := newFixture(t)
	note, tag := uuid.NewString(), uuid.NewString()
	f.mutate(t, 1, storetest.InsertNote(note, f.id.Scope, "first"))
	f.mutate(t, 2, func(tx pgx.Tx) error {
		if err := storetest.TagNote(tag, note, "home")(tx); err != nil {
			return err
		}
		return storetest.RenameNote(note, "second")(tx)
	})
	f.mutate(t, 3, storetest.DeleteNote(note))

	resp := f.pull(t, pull.Request{})
	if resp.GetHeadVersion() != 3 || resp.GetEpoch() != 1 || resp.GetLastMutationId() != 3 {
		t.Fatalf("response %v", resp)
	}
	got := entries(t, resp)
	if got.GetMore() || len(got.GetVersions()) != 3 {
		t.Fatalf("entries %v", got)
	}
	for i, v := range got.GetVersions() {
		if v.GetVersion() != uint64(i+1) || v.GetMutationId() != uint64(i+1) || v.GetClientId() != f.id.Client || len(v.GetHash()) != 32 {
			t.Fatalf("version %d: %v", i, v)
		}
		if v.GetCommittedAt() < time.Now().Add(-time.Minute).UnixMilli() {
			t.Fatalf("committed_at %d is not unix milliseconds", v.GetCommittedAt())
		}
	}
	second := got.GetVersions()[1].GetChanges()
	if len(second) != 2 || second[0].GetTable() != "note_tag" || second[1].GetTable() != "note" {
		t.Fatalf("changes %v", second)
	}
	var row map[string]any
	if err := json.Unmarshal(second[1].GetData(), &row); err != nil {
		t.Fatal(err)
	}
	if row["title"] != "second" || row["id"] != note {
		t.Fatalf("row %v", row)
	}
	if _, ok := row["secret"]; ok {
		t.Fatal("an excluded column was served")
	}
	deletes := got.GetVersions()[2].GetChanges()
	if len(deletes) != 2 {
		t.Fatalf("deletes %v", deletes)
	}
	for _, c := range deletes {
		if c.GetOp() != storev1.Change_OP_DELETE || len(c.GetData()) != 0 {
			t.Fatalf("delete %v", c)
		}
	}
}

func TestPullContinuesFromTheCursor(t *testing.T) {
	f := newFixture(t)
	for i := uint64(1); i <= 4; i++ {
		f.mutate(t, i, storetest.InsertNote(uuid.NewString(), f.id.Scope, "n"))
	}
	first := entries(t, f.pull(t, pull.Request{Limit: 2}))
	if !first.GetMore() || len(first.GetVersions()) != 2 {
		t.Fatalf("first page %v", first)
	}
	second := entries(t, f.pull(t, pull.Request{Cursor: cursorAt(first.GetVersions()[1], 1), Limit: 2}))
	if second.GetMore() || len(second.GetVersions()) != 2 || second.GetVersions()[0].GetVersion() != 3 {
		t.Fatalf("second page %v", second)
	}
	last := entries(t, f.pull(t, pull.Request{Cursor: cursorAt(second.GetVersions()[1], 1)}))
	if last.GetMore() || len(last.GetVersions()) != 0 {
		t.Fatalf("caught-up page %v", last)
	}
}

func TestPullCapsTheLimit(t *testing.T) {
	f := newFixture(t)
	f.serverVersions(t, 1001)
	byDefault := entries(t, f.pull(t, pull.Request{}))
	if len(byDefault.GetVersions()) != pull.DefaultLimit || !byDefault.GetMore() {
		t.Fatalf("default page has %d versions", len(byDefault.GetVersions()))
	}
	capped := entries(t, f.pull(t, pull.Request{Limit: 5000}))
	if len(capped.GetVersions()) != pull.MaxLimit || !capped.GetMore() {
		t.Fatalf("capped page has %d versions", len(capped.GetVersions()))
	}
	rest := entries(t, f.pull(t, pull.Request{Cursor: cursorAt(capped.GetVersions()[pull.MaxLimit-1], 1), Limit: 5000}))
	if len(rest.GetVersions()) != 1 || rest.GetMore() {
		t.Fatalf("rest %v", rest)
	}
}

func TestPullServesRejections(t *testing.T) {
	f := newFixture(t)
	f.mutate(t, 1, storetest.InsertNote(uuid.NewString(), f.id.Scope, "ok"))
	f.mutate(t, 2, func(tx pgx.Tx) error {
		if err := storetest.InsertNote(uuid.NewString(), f.id.Scope, "nope")(tx); err != nil {
			return err
		}
		return storetest.Rejection{Reason: `quota "reached"`}
	})
	resp := f.pull(t, pull.Request{})
	v := entries(t, resp).GetVersions()[1]
	if v.GetRejection().GetMutationId() != 2 || v.GetRejection().GetReason() != `quota "reached"` || len(v.GetChanges()) != 0 {
		t.Fatalf("rejection version %v", v)
	}
	if entries(t, resp).GetVersions()[0].GetRejection() != nil {
		t.Fatal("an applied version carries a rejection")
	}
	if resp.GetLastMutationId() != 2 {
		t.Fatalf("last_mutation_id %d", resp.GetLastMutationId())
	}
}

func TestPullServesServerWrites(t *testing.T) {
	f := newFixture(t)
	if _, err := f.writer.ServerWrite(context.Background(), f.id.Scope, storetest.InsertNote(uuid.NewString(), f.id.Scope, "import")); err != nil {
		t.Fatal(err)
	}
	resp := f.pull(t, pull.Request{})
	v := entries(t, resp).GetVersions()[0]
	if v.GetClientId() != "00000000-0000-0000-0000-000000000000" || len(v.GetChanges()) != 1 {
		t.Fatalf("server version %v", v)
	}
	if resp.GetLastMutationId() != 0 {
		t.Fatalf("last_mutation_id %d", resp.GetLastMutationId())
	}
}

func TestPullResets(t *testing.T) {
	f := newFixture(t)
	f.mutate(t, 1, storetest.InsertNote(uuid.NewString(), f.id.Scope, "a"))
	f.mutate(t, 2, storetest.InsertNote(uuid.NewString(), f.id.Scope, "b"))
	versions := entries(t, f.pull(t, pull.Request{})).GetVersions()
	at1 := cursorAt(versions[0], 1)

	cases := map[string]struct {
		cursor pull.Cursor
		want   string
	}{
		"epoch differs":    {pull.Cursor{Epoch: 7, Version: 1, Hash: at1.Hash}, pull.ReasonEpochChanged},
		"hash mismatch":    {pull.Cursor{Epoch: 1, Version: 1, Hash: versions[1].GetHash()}, pull.ReasonHashMismatch},
		"empty hash":       {pull.Cursor{Epoch: 1, Version: 1}, pull.ReasonHashMismatch},
		"hash at zero":     {pull.Cursor{Epoch: 1, Hash: at1.Hash}, pull.ReasonHashMismatch},
		"ahead of head":    {pull.Cursor{Epoch: 1, Version: 3, Hash: at1.Hash}, pull.ReasonAheadOfHead},
		"version no epoch": {pull.Cursor{Version: 1, Hash: at1.Hash}, pull.ReasonEpochChanged},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp := f.pull(t, pull.Request{Cursor: tc.cursor, MinVersion: 1})
			reset := resp.GetReset_()
			if reset == nil || reset.GetReason() != tc.want || reset.GetEpoch() != 1 {
				t.Fatalf("got %v, want reset %q", resp, tc.want)
			}
			if resp.GetHeadVersion() != 2 || resp.GetLastMutationId() != 2 || resp.GetEpoch() != 1 {
				t.Fatalf("response %v", resp)
			}
		})
	}
	if got := entries(t, f.pull(t, pull.Request{Cursor: at1})); len(got.GetVersions()) != 1 {
		t.Fatalf("matching cursor: %v", got)
	}
}

func TestPullResetsAfterAnEpochBump(t *testing.T) {
	f := newFixture(t)
	f.mutate(t, 1, storetest.InsertNote(uuid.NewString(), f.id.Scope, "a"))
	at1 := cursorAt(entries(t, f.pull(t, pull.Request{})).GetVersions()[0], 1)
	if _, err := f.db.Pool.Exec(context.Background(), "UPDATE store_scope SET epoch = 2 WHERE scope_id = $1", f.id.Scope); err != nil {
		t.Fatal(err)
	}
	resp := f.pull(t, pull.Request{Cursor: at1})
	if resp.GetReset_().GetReason() != pull.ReasonEpochChanged || resp.GetReset_().GetEpoch() != 2 || resp.GetEpoch() != 2 {
		t.Fatalf("got %v", resp)
	}
	again := entries(t, f.pull(t, pull.Request{Cursor: pull.Cursor{Epoch: 2}}))
	if len(again.GetVersions()) != 1 {
		t.Fatalf("after reset %v", again)
	}
}

func TestPullOfAnUnknownScopeIsEmpty(t *testing.T) {
	f := newFixture(t)
	resp := f.pull(t, pull.Request{})
	if resp.GetEpoch() != 1 || resp.GetHeadVersion() != 0 || len(entries(t, resp).GetVersions()) != 0 {
		t.Fatalf("got %v", resp)
	}
}

func TestPullIsScopedToTheToken(t *testing.T) {
	f := newFixture(t)
	f.mutate(t, 1, storetest.InsertNote(uuid.NewString(), f.id.Scope, "mine"))
	other := auth.Identity{Scope: uuid.NewString(), Client: f.id.Client}
	resp, err := f.service.Pull(context.Background(), other, pull.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetHeadVersion() != 0 || resp.GetLastMutationId() != 0 || len(entries(t, resp).GetVersions()) != 0 {
		t.Fatalf("another scope saw %v", resp)
	}
}

func TestPullWaitsForMinVersion(t *testing.T) {
	f := newFixture(t)
	f.mutate(t, 1, storetest.InsertNote(uuid.NewString(), f.id.Scope, "a"))
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = f.writer.Mutate(context.Background(), f.id.Scope, f.id.Client, 2, storetest.InsertNote(uuid.NewString(), f.id.Scope, "b"))
	}()
	resp := f.pull(t, pull.Request{MinVersion: 2})
	if resp.GetHeadVersion() != 2 || len(entries(t, resp).GetVersions()) != 2 {
		t.Fatalf("got %v", resp)
	}
}

func TestPullGivesUpWhenMinVersionNeverArrives(t *testing.T) {
	f := newFixture(t)
	f.mutate(t, 1, storetest.InsertNote(uuid.NewString(), f.id.Scope, "a"))
	start := time.Now()
	_, err := f.service.Pull(context.Background(), f.id, pull.Request{MinVersion: 5})
	if !errors.Is(err, pull.ErrBehind) {
		t.Fatalf("got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("waited %s", elapsed)
	}
}
