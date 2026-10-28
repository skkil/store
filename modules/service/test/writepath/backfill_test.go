package writepath_test

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/skkil/store/modules/service/internal/storetest"
)

func bypassed(t *testing.T, db *storetest.Database, sql string, args ...any) {
	t.Helper()
	inTx(t, db, func(ctx context.Context, tx pgx.Tx) {
		exec(t, ctx, tx, "SET LOCAL store.bypass = on")
		exec(t, ctx, tx, sql, args...)
	})
}

func captureExisting(t *testing.T, ctx context.Context, tx pgx.Tx, args ...any) int {
	t.Helper()
	sql := "SELECT store_capture_existing($1)"
	if len(args) == 3 {
		sql = "SELECT store_capture_existing($1, $2, $3)"
	}
	var n int
	if err := tx.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCaptureExistingLogsUnloggedRowsInIDOrder(t *testing.T) {
	db := storetest.NewDatabase(t)
	scope, other := uuid.NewString(), uuid.NewString()
	ids := []string{uuid.NewString(), uuid.NewString(), uuid.NewString()}
	for i, id := range ids {
		bypassed(t, db, "INSERT INTO note (id, scope_id, title, secret) VALUES ($1, $2, $3, 'hidden')", id, scope, "n"+string(rune('a'+i)))
	}
	bypassed(t, db, "INSERT INTO note (id, scope_id, title) VALUES ($1, $2, 'not mine')", uuid.NewString(), other)
	inTx(t, db, func(ctx context.Context, tx pgx.Tx) {
		exec(t, ctx, tx, "SELECT store_begin_server($1)", scope)
		if n := captureExisting(t, ctx, tx, "note", "scope_id", []string{scope}); n != 3 {
			t.Fatalf("captured %d rows", n)
		}
		exec(t, ctx, tx, "SELECT store_end()")
	})
	log := readLog(t, db.Pool, scope)
	if len(log) != 4 || log[0].Kind != "commit" || log[0].ClientID != serverClient {
		t.Fatalf("log %+v", log)
	}
	slices.Sort(ids)
	for i, row := range log[1:] {
		if row.Kind != "patch" || row.Seq != i+1 || *row.Table != "note" || *row.Op != "upsert" || *row.RowID != ids[i] {
			t.Fatalf("patch %d: %+v", i, row)
		}
		if strings.Contains(*row.Data, "secret") || !strings.Contains(*row.Data, `"id": "`+ids[i]+`"`) {
			t.Fatalf("image %s", *row.Data)
		}
	}
}

func TestCaptureExistingMatchesTheTriggersImages(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	scope := uuid.NewString()
	note := uuid.NewString()
	mustMutate(t, w, scope, uuid.NewString(), 1, storetest.InsertNote(note, scope, "same"))
	if _, err := w.ServerWrite(context.Background(), scope, func(tx pgx.Tx) error {
		captureExisting(t, context.Background(), tx, "note", "id", []string{note})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	log := readLog(t, db.Pool, scope)
	if len(log) != 4 || *log[1].Data != *log[3].Data || *log[1].RowID != *log[3].RowID {
		t.Fatalf("log %+v", log)
	}
}

func TestCaptureExistingFollowsCapturedChangesAndKeepsTheChain(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	ctx := context.Background()
	scope, client := uuid.NewString(), uuid.NewString()
	old, tag := uuid.NewString(), uuid.NewString()
	mustMutate(t, w, scope, client, 1, storetest.InsertNote(uuid.NewString(), scope, "logged"))
	bypassed(t, db, "INSERT INTO note (id, scope_id, title) VALUES ($1, $2, 'old')", old, scope)
	bypassed(t, db, "INSERT INTO note_tag (id, note_id, tag) VALUES ($1, $2, 'old')", tag, old)
	if _, err := w.ServerWrite(ctx, scope, func(tx pgx.Tx) error {
		if err := storetest.InsertNote(uuid.NewString(), scope, "first")(tx); err != nil {
			return err
		}
		captureExisting(t, ctx, tx, "note", "scope_id", []string{scope})
		captureExisting(t, ctx, tx, "note_tag", "note_id", []string{old})
		return storetest.RenameNote(old, "renamed")(tx)
	}); err != nil {
		t.Fatal(err)
	}
	mustMutate(t, w, scope, client, 2, storetest.DeleteNote(old))

	log := readLog(t, db.Pool, scope)
	var seqs []int
	for _, row := range log {
		if row.Version == 2 && row.Kind == "patch" {
			seqs = append(seqs, row.Seq)
		}
	}
	if !slices.Equal(seqs, []int{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("seqs %v", seqs)
	}
	prev := []byte{}
	for i := 0; i < len(log); {
		header := log[i]
		var patches []logRow
		for i++; i < len(log) && log[i].Version == header.Version; i++ {
			patches = append(patches, log[i])
		}
		prev = chainHash(prev, header, patches)
		if !bytes.Equal(header.Hash, prev) {
			t.Fatalf("version %d: hash %x, independently %x", header.Version, header.Hash, prev)
		}
	}
	if version, headHash := head(t, db.Pool, scope); version != 3 || !bytes.Equal(headHash, prev) {
		t.Fatalf("head %d %x, chain %x", version, headHash, prev)
	}
}

func TestCaptureExistingWithoutAFilterTakesEveryRow(t *testing.T) {
	db := storetest.NewDatabase(t)
	scope := uuid.NewString()
	for range 2 {
		bypassed(t, db, "INSERT INTO note (id, scope_id, title) VALUES ($1, $2, 'x')", uuid.NewString(), uuid.NewString())
	}
	inTx(t, db, func(ctx context.Context, tx pgx.Tx) {
		exec(t, ctx, tx, "SELECT store_begin_server($1)", scope)
		if n := captureExisting(t, ctx, tx, "note"); n != 2 {
			t.Fatalf("captured %d", n)
		}
		if n := captureExisting(t, ctx, tx, "note_tag"); n != 0 {
			t.Fatalf("captured %d tags", n)
		}
		exec(t, ctx, tx, "SELECT store_end()")
	})
}

func TestCaptureExistingRefusesBadArguments(t *testing.T) {
	db := storetest.NewDatabase(t)
	ctx := context.Background()
	scope := uuid.NewString()
	inTx(t, db, func(ctx context.Context, tx pgx.Tx) {
		exec(t, ctx, tx, "CREATE TABLE unsynced (id uuid PRIMARY KEY)")
	})
	cases := map[string]struct {
		sql  string
		args []any
	}{
		"unregistered table":   {"SELECT store_capture_existing('unsynced')", nil},
		"unknown column":       {"SELECT store_capture_existing('note', 'nope', $1)", []any{[]string{scope}}},
		"column without value": {"SELECT store_capture_existing('note', 'scope_id', NULL)", nil},
		"values without column": {
			"SELECT store_capture_existing('note', NULL, $1)", []any{[]string{scope}},
		},
		"injection as column": {
			"SELECT store_capture_existing('note', 'scope_id = scope_id OR true --', $1)", []any{[]string{scope}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tx, err := db.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			exec(t, ctx, tx, "SELECT store_begin_server($1)", scope)
			if _, err := tx.Exec(ctx, tc.sql, tc.args...); sqlState(err) != "22023" {
				t.Fatalf("got %v, want SQLSTATE 22023", err)
			}
		})
	}
	if _, err := db.Pool.Exec(ctx, "SELECT store_capture_existing('note')"); sqlState(err) != "55000" {
		t.Fatalf("outside a mutation: %v", err)
	}
}

func TestCaptureExistingTreatsValuesAsData(t *testing.T) {
	db := storetest.NewDatabase(t)
	scope := uuid.NewString()
	bypassed(t, db, "INSERT INTO note (id, scope_id, title) VALUES ($1, $2, 'x')", uuid.NewString(), scope)
	tx, err := db.Pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	exec(t, context.Background(), tx, "SELECT store_begin_server($1)", scope)
	for value, want := range map[string]int{"x' OR true --": 0, "x": 1} {
		if n := captureExisting(t, context.Background(), tx, "note", "title", []string{value}); n != want {
			t.Fatalf("%q captured %d, want %d", value, n, want)
		}
	}
}

func TestBumpEpochMovesAnExistingScope(t *testing.T) {
	db := storetest.NewDatabase(t)
	ctx := context.Background()
	scope := uuid.NewString()
	var epoch *int32
	if err := db.Pool.QueryRow(ctx, "SELECT store_bump_epoch($1)", scope).Scan(&epoch); err != nil || epoch != nil {
		t.Fatalf("unknown scope: %v %v", epoch, err)
	}
	mustMutate(t, storetest.Writer{Pool: db.Pool}, scope, uuid.NewString(), 1, storetest.InsertNote(uuid.NewString(), scope, "x"))
	for want := int32(2); want <= 3; want++ {
		if err := db.Pool.QueryRow(ctx, "SELECT store_bump_epoch($1)", scope).Scan(&epoch); err != nil || epoch == nil || *epoch != want {
			t.Fatalf("epoch %v %v, want %d", epoch, err, want)
		}
	}
	if v, _ := head(t, db.Pool, scope); v != 1 {
		t.Fatalf("head %d", v)
	}
}
