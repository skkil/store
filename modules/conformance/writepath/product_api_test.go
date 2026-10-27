package writepath_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/skkil/store/modules/conformance/internal/stacktest"
	"github.com/skkil/store/modules/conformance/writepath"
)

func TestEndIfChangedLogsOnlyWritesThatChangedRows(t *testing.T) {
	db := stacktest.NewDatabase(t)
	ctx := context.Background()
	scope := uuid.New()
	for _, title := range []string{"", "imported"} {
		err := pgx.BeginFunc(ctx, db.Pool, func(tx pgx.Tx) error {
			if err := writepath.BeginServer(ctx, tx, scope); err != nil {
				return err
			}
			if title != "" {
				if _, err := tx.Exec(ctx, "INSERT INTO todo (id, scope_id, title) VALUES ($1, $2, $3)", uuid.Must(uuid.NewV7()), scope, title); err != nil {
					return err
				}
			}
			version, ok, err := writepath.EndIfChanged(ctx, tx)
			if err != nil {
				return err
			}
			if ok != (title != "") || (ok && version != 1) {
				t.Fatalf("title %q: version %d, ok %v", title, version, ok)
			}
			open, err := writepath.IsOpen(ctx, tx)
			if err != nil || open {
				t.Fatalf("still open after EndIfChanged: %v %v", open, err)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := countRows(t, db, "SELECT count(*) FROM store_log WHERE scope_id = $1 AND seq = 0", scope); n != 1 {
		t.Fatalf("%d versions, want 1", n)
	}
}

func TestAbandonClosesAnEmptyServerWrite(t *testing.T) {
	db := stacktest.NewDatabase(t)
	ctx := context.Background()
	scope := uuid.New()
	err := pgx.BeginFunc(ctx, db.Pool, func(tx pgx.Tx) error {
		if err := writepath.BeginServer(ctx, tx, scope); err != nil {
			return err
		}
		return writepath.Abandon(ctx, tx)
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM store_log WHERE scope_id = $1", scope); n != 0 {
		t.Fatalf("%d log rows for an abandoned write", n)
	}
}

func TestClaimClientAndDropScope(t *testing.T) {
	db := stacktest.NewDatabase(t)
	w := writepath.New(db.Pool, discard)
	ctx := context.Background()
	scope, other, client := uuid.New(), uuid.New(), uuid.New()
	for _, tc := range []struct {
		scope uuid.UUID
		want  bool
	}{{scope, true}, {scope, true}, {other, false}} {
		ok, err := writepath.ClaimClient(ctx, db.Pool, tc.scope, client)
		if err != nil || ok != tc.want {
			t.Fatalf("claim in %s: %v %v, want %v", tc.scope, ok, err, tc.want)
		}
	}
	if _, err := w.Apply(ctx, scope, client, 1, insertTodo("a")); err != nil {
		t.Fatal(err)
	}
	_, err := w.ApplyServer(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "DELETE FROM todo WHERE scope_id = $1", scope)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writepath.DropScope(ctx, db.Pool, scope); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM store_client WHERE client_id = $1", client); n != 0 {
		t.Fatal("the dropped scope's client stayed")
	}
	if ok, err := writepath.ClaimClient(ctx, db.Pool, other, client); err != nil || !ok {
		t.Fatalf("claim after drop: %v %v", ok, err)
	}
}
