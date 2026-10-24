package writepath_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/skkil/store/modules/service/internal/storetest"
)

func inTx(t *testing.T, db *storetest.Database, body func(ctx context.Context, tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	body(ctx, tx)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func exec(t *testing.T, ctx context.Context, tx pgx.Tx, sql string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func isOpen(t *testing.T, ctx context.Context, tx pgx.Tx) bool {
	t.Helper()
	var open bool
	if err := tx.QueryRow(ctx, "SELECT store_is_open()").Scan(&open); err != nil {
		t.Fatal(err)
	}
	return open
}

func TestEndIfChangedClosesWithoutAVersionWhenNothingChanged(t *testing.T) {
	db := storetest.NewDatabase(t)
	scope := uuid.NewString()
	inTx(t, db, func(ctx context.Context, tx pgx.Tx) {
		exec(t, ctx, tx, "SELECT store_begin_server($1)", scope)
		var version *int64
		if err := tx.QueryRow(ctx, "SELECT store_end_if_changed()").Scan(&version); err != nil {
			t.Fatal(err)
		}
		if version != nil {
			t.Fatalf("version %d for an empty write", *version)
		}
		if isOpen(t, ctx, tx) {
			t.Fatal("the write is still open")
		}
	})
	if v, _ := head(t, db.Pool, scope); v != 0 {
		t.Fatalf("head %d", v)
	}
	if log := readLog(t, db.Pool, scope); len(log) != 0 {
		t.Fatalf("log %+v", log)
	}
}

func TestEndIfChangedLogsAVersionWhenRowsChanged(t *testing.T) {
	db := storetest.NewDatabase(t)
	scope := uuid.NewString()
	inTx(t, db, func(ctx context.Context, tx pgx.Tx) {
		exec(t, ctx, tx, "SELECT store_begin_server($1)", scope)
		if err := storetest.InsertNote(uuid.NewString(), scope, "server")(tx); err != nil {
			t.Fatal(err)
		}
		var version *int64
		if err := tx.QueryRow(ctx, "SELECT store_end_if_changed()").Scan(&version); err != nil {
			t.Fatal(err)
		}
		if version == nil || *version != 1 {
			t.Fatalf("version %v", version)
		}
	})
	log := readLog(t, db.Pool, scope)
	if len(log) != 2 || log[0].Kind != "commit" || log[0].ClientID != serverClient {
		t.Fatalf("log %+v", log)
	}
}

func TestAbandonClosesAnUnchangedServerWrite(t *testing.T) {
	db := storetest.NewDatabase(t)
	scope := uuid.NewString()
	note := uuid.NewString()
	if _, err := (storetest.Writer{Pool: db.Pool}).ServerWrite(context.Background(), scope, storetest.InsertNote(note, scope, "same")); err != nil {
		t.Fatal(err)
	}
	inTx(t, db, func(ctx context.Context, tx pgx.Tx) {
		exec(t, ctx, tx, "SELECT store_begin_server($1)", scope)
		exec(t, ctx, tx, "SAVEPOINT work")
		if err := storetest.RenameNote(note, "changed")(tx); err != nil {
			t.Fatal(err)
		}
		exec(t, ctx, tx, "ROLLBACK TO SAVEPOINT work")
		exec(t, ctx, tx, "SELECT store_abandon()")
		if isOpen(t, ctx, tx) {
			t.Fatal("the write is still open")
		}
	})
	if v, _ := head(t, db.Pool, scope); v != 1 {
		t.Fatalf("head %d", v)
	}
}

func TestAbandonRefusesCapturedChangesAndDeviceMutations(t *testing.T) {
	db := storetest.NewDatabase(t)
	ctx := context.Background()
	scope := uuid.NewString()
	cases := map[string]func(tx pgx.Tx) error{
		"captured changes": func(tx pgx.Tx) error {
			exec(t, ctx, tx, "SELECT store_begin_server($1)", scope)
			if err := storetest.InsertNote(uuid.NewString(), scope, "kept")(tx); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, "SELECT store_abandon()")
			return err
		},
		"device mutation abandoned": func(tx pgx.Tx) error {
			exec(t, ctx, tx, "SELECT store_begin($1, $2, 1)", scope, uuid.NewString())
			_, err := tx.Exec(ctx, "SELECT store_abandon()")
			return err
		},
		"device mutation ended if changed": func(tx pgx.Tx) error {
			exec(t, ctx, tx, "SELECT store_begin($1, $2, 1)", scope, uuid.NewString())
			_, err := tx.Exec(ctx, "SELECT store_end_if_changed()")
			return err
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			tx, err := db.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if state := sqlState(run(tx)); state != "22023" {
				t.Fatalf("SQLSTATE %q, want 22023", state)
			}
		})
	}
	_, err := db.Pool.Exec(ctx, "SELECT store_abandon()")
	if sqlState(err) != "55000" {
		t.Fatalf("abandon with nothing open: %v", err)
	}
}

func TestIsOpenFollowsTheMutation(t *testing.T) {
	db := storetest.NewDatabase(t)
	inTx(t, db, func(ctx context.Context, tx pgx.Tx) {
		if isOpen(t, ctx, tx) {
			t.Fatal("open before begin")
		}
		exec(t, ctx, tx, "SELECT store_begin($1, $2, 1)", uuid.NewString(), uuid.NewString())
		if !isOpen(t, ctx, tx) {
			t.Fatal("closed after begin")
		}
		exec(t, ctx, tx, "SELECT store_end()")
		if isOpen(t, ctx, tx) {
			t.Fatal("open after end")
		}
	})
}

func TestClaimClientBindsAClientToOneScope(t *testing.T) {
	db := storetest.NewDatabase(t)
	ctx := context.Background()
	scope, other, client := uuid.NewString(), uuid.NewString(), uuid.NewString()
	claim := func(scope, client string) (bool, error) {
		var ok bool
		err := db.Pool.QueryRow(ctx, "SELECT store_claim_client($1, $2)", scope, client).Scan(&ok)
		return ok, err
	}
	for _, want := range []struct {
		scope string
		ok    bool
	}{{scope, true}, {scope, true}, {other, false}} {
		ok, err := claim(want.scope, client)
		if err != nil || ok != want.ok {
			t.Fatalf("claim for %s: %v %v, want %v", want.scope, ok, err, want.ok)
		}
	}
	if _, err := claim(scope, serverClient); sqlState(err) != "22023" {
		t.Fatalf("claiming the server client: %v", err)
	}
	if last := lastMutation(t, db.Pool, client); last != 0 {
		t.Fatalf("last_mutation_id %d", last)
	}
	result := mustMutate(t, storetest.Writer{Pool: db.Pool}, scope, client, 1, func(pgx.Tx) error { return nil })
	if result.Status != storetest.Applied {
		t.Fatalf("a claimed client's first mutation: %+v", result)
	}
	if _, err := (storetest.Writer{Pool: db.Pool}).Mutate(ctx, other, client, 2, func(pgx.Tx) error { return nil }); sqlState(err) != "42501" {
		t.Fatalf("pushing to another scope: %v", err)
	}
}

func TestDropScopeRemovesItsClientsAndLog(t *testing.T) {
	db := storetest.NewDatabase(t)
	ctx := context.Background()
	scope, kept, client := uuid.NewString(), uuid.NewString(), uuid.NewString()
	w := storetest.Writer{Pool: db.Pool}
	note := uuid.NewString()
	mustMutate(t, w, scope, client, 1, storetest.InsertNote(note, scope, "gone"))
	mustMutate(t, w, kept, uuid.NewString(), 1, storetest.InsertNote(uuid.NewString(), kept, "kept"))
	mustMutate(t, w, scope, client, 2, storetest.DeleteNote(note))
	if _, err := db.Pool.Exec(ctx, "SELECT store_drop_scope($1)", scope); err != nil {
		t.Fatal(err)
	}
	var scopes, clients, logs int
	err := db.Pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM store_scope WHERE scope_id = $1),
		       (SELECT count(*) FROM store_client WHERE scope_id = $1),
		       (SELECT count(*) FROM store_log WHERE scope_id = $1)`, scope).Scan(&scopes, &clients, &logs)
	if err != nil {
		t.Fatal(err)
	}
	if scopes+clients+logs != 0 {
		t.Fatalf("left %d scopes, %d clients, %d log rows", scopes, clients, logs)
	}
	if v, _ := head(t, db.Pool, kept); v != 1 {
		t.Fatalf("another scope's head %d", v)
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec(t, ctx, tx, "SELECT store_begin_server($1)", kept)
	if _, err := tx.Exec(ctx, "SELECT store_drop_scope($1)", kept); sqlState(err) != "55000" {
		t.Fatalf("dropping the open scope: %v", err)
	}
}

func TestUpdatesOfExcludedColumnsOnlyAreNotLogged(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	scope, client := uuid.NewString(), uuid.NewString()
	note := uuid.NewString()
	mustMutate(t, w, scope, client, 1, storetest.InsertNote(note, scope, "x"))
	mustMutate(t, w, scope, client, 2, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), "UPDATE note SET secret = 'rotated' WHERE id = $1", note)
		return err
	})
	log := readLog(t, db.Pool, scope)
	if len(log) != 3 || log[2].Version != 2 || log[2].Kind != "commit" {
		t.Fatalf("log %+v", log)
	}
}
