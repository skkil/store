package writepath_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/skkil/store/modules/conformance/internal/stacktest"
	"github.com/skkil/store/modules/conformance/writepath"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func insertTodo(title string) writepath.Body {
	return func(ctx context.Context, m *writepath.Mutation) error {
		_, err := m.Tx.Exec(ctx, "INSERT INTO todo (id, scope_id, title) VALUES ($1, $2, $3)", m.NextID(), m.Scope, title)
		return err
	}
}

func countRows(t *testing.T, db *stacktest.Database, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := db.Pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestApplyCommitsOnceAndAnswersDuplicates(t *testing.T) {
	db := stacktest.NewDatabase(t)
	w := writepath.New(db.Pool, discard)
	ctx := context.Background()
	scope, client := uuid.New(), uuid.New()

	first, err := w.Apply(ctx, scope, client, 1, insertTodo("a"))
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != writepath.Applied || first.Version != 1 {
		t.Fatalf("first apply = %+v, want applied v1", first)
	}
	again, err := w.Apply(ctx, scope, client, 1, insertTodo("a"))
	if err != nil {
		t.Fatal(err)
	}
	if again.Status != writepath.Duplicate || again.Version != 1 {
		t.Fatalf("second apply = %+v, want duplicate v1", again)
	}
	if n := countRows(t, db, "SELECT count(*) FROM todo WHERE scope_id = $1", scope); n != 1 {
		t.Fatalf("%d todos, want 1", n)
	}
	if n := countRows(t, db, "SELECT count(*) FROM store_log WHERE scope_id = $1 AND seq = 0", scope); n != 1 {
		t.Fatalf("%d versions, want 1", n)
	}
}

func TestRejectionRollsBackTheBodyAndRecordsTheReason(t *testing.T) {
	db := stacktest.NewDatabase(t)
	w := writepath.New(db.Pool, discard)
	ctx := context.Background()
	scope, client := uuid.New(), uuid.New()

	result, err := w.Apply(ctx, scope, client, 1, func(ctx context.Context, m *writepath.Mutation) error {
		if err := insertTodo("half")(ctx, m); err != nil {
			return err
		}
		return writepath.Reject("no halves")
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != writepath.Rejected || result.Version != 1 || result.Reason != "no halves" {
		t.Fatalf("result = %+v, want rejected v1 with the reason", result)
	}
	if n := countRows(t, db, "SELECT count(*) FROM todo WHERE scope_id = $1", scope); n != 0 {
		t.Fatalf("%d todos survived the rejection", n)
	}
	var kind, reason string
	if err := db.Pool.QueryRow(ctx, "SELECT kind, data->>'reason' FROM store_log WHERE scope_id = $1 AND version = 1 AND seq = 0", scope).Scan(&kind, &reason); err != nil {
		t.Fatal(err)
	}
	if kind != "rejection" || reason != "no halves" {
		t.Fatalf("log entry = %s %q, want rejection \"no halves\"", kind, reason)
	}
	if n := countRows(t, db, "SELECT count(*) FROM store_log WHERE scope_id = $1 AND kind = 'patch'", scope); n != 0 {
		t.Fatalf("%d row changes logged for a rejection", n)
	}
}

func TestOtherErrorsAbortWithoutRecordingTheMutation(t *testing.T) {
	db := stacktest.NewDatabase(t)
	w := writepath.New(db.Pool, discard)
	ctx := context.Background()
	scope, client := uuid.New(), uuid.New()
	boom := errors.New("disk on fire")

	if _, err := w.Apply(ctx, scope, client, 1, func(context.Context, *writepath.Mutation) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if n := countRows(t, db, "SELECT count(*) FROM store_client WHERE client_id = $1", client); n != 0 {
		t.Fatal("an aborted mutation must leave no trace, so the retry applies it")
	}
	retry, err := w.Apply(ctx, scope, client, 1, insertTodo("retry"))
	if err != nil {
		t.Fatal(err)
	}
	if retry.Status != writepath.Applied {
		t.Fatalf("retry = %+v, want applied", retry)
	}
}

func TestApplyServerRecordsTheWriteUnderTheServerClient(t *testing.T) {
	db := stacktest.NewDatabase(t)
	w := writepath.New(db.Pool, discard)
	ctx := context.Background()
	scope := uuid.New()

	version, err := w.ApplyServer(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO todo (id, scope_id, title) VALUES ($1, $2, 'imported')", uuid.Must(uuid.NewV7()), scope)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var client string
	if err := db.Pool.QueryRow(ctx, "SELECT client_id::text FROM store_log WHERE scope_id = $1 AND version = $2 AND seq = 0", scope, int64(version)).Scan(&client); err != nil {
		t.Fatal(err)
	}
	if client != "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("server write recorded under %s", client)
	}
}

func TestApplyPokesAfterCommit(t *testing.T) {
	db := stacktest.NewDatabase(t)
	w := writepath.New(db.Pool, discard)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	listener, err := pgx.Connect(ctx, db.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close(context.Background())
	if _, err := listener.Exec(ctx, "LISTEN store_poke"); err != nil {
		t.Fatal(err)
	}
	scope := uuid.New()
	if _, err := w.Apply(ctx, scope, uuid.New(), 1, insertTodo("poked")); err != nil {
		t.Fatal(err)
	}
	notification, err := listener.WaitForNotification(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := scope.String() + ":1"; notification.Payload != want {
		t.Fatalf("poke = %q, want %q", notification.Payload, want)
	}
}

func TestMutationNextIDCountsUp(t *testing.T) {
	client := uuid.New()
	m := &writepath.Mutation{Client: client, ID: 7}
	for n := range uint32(3) {
		if got, want := m.NextID(), writepath.DeriveID(client, 7, n); got != want {
			t.Fatalf("id %d = %s, want %s", n, got, want)
		}
	}
}
