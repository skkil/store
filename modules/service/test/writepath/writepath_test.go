// Package writepath_test exercises modules/sql's write path directly on Postgres.
package writepath_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skkil/store/modules/service/internal/storetest"
)

const serverClient = "00000000-0000-0000-0000-000000000000"

type logRow struct {
	Version    int64
	Seq        int
	Kind       string
	Table      *string
	RowID      *string
	Op         *string
	Data       *string
	ClientID   string
	MutationID int64
	Hash       []byte
}

func readLog(t *testing.T, pool *pgxpool.Pool, scope string) []logRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT version, seq, kind, table_name, row_id, op, data::text, client_id::text, mutation_id, hash
		  FROM store_log WHERE scope_id = $1 ORDER BY version, seq`, scope)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (logRow, error) {
		var r logRow
		err := row.Scan(&r.Version, &r.Seq, &r.Kind, &r.Table, &r.RowID, &r.Op, &r.Data, &r.ClientID, &r.MutationID, &r.Hash)
		return r, err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func head(t *testing.T, pool *pgxpool.Pool, scope string) (int64, []byte) {
	t.Helper()
	var version int64
	var hash []byte
	err := pool.QueryRow(context.Background(), "SELECT head_version, head_hash FROM store_scope WHERE scope_id = $1", scope).Scan(&version, &hash)
	if err != nil {
		t.Fatal(err)
	}
	return version, hash
}

func lastMutation(t *testing.T, pool *pgxpool.Pool, client string) int64 {
	t.Helper()
	var last int64
	if err := pool.QueryRow(context.Background(), "SELECT last_mutation_id FROM store_client WHERE client_id = $1", client).Scan(&last); err != nil {
		t.Fatal(err)
	}
	return last
}

func mustMutate(t *testing.T, w storetest.Writer, scope, client string, id uint64, body func(pgx.Tx) error) storetest.Result {
	t.Helper()
	result, err := w.Mutate(context.Background(), scope, client, id, body)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func TestMutationsGetDenseVersionsAndAck(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	scope, client := uuid.NewString(), uuid.NewString()
	note := uuid.NewString()
	for i, body := range []func(pgx.Tx) error{
		storetest.InsertNote(note, scope, "one"),
		storetest.RenameNote(note, "two"),
		storetest.RenameNote(note, "three"),
	} {
		result := mustMutate(t, w, scope, client, uint64(i+1), body)
		if result != (storetest.Result{Status: storetest.Applied, Version: uint64(i + 1)}) {
			t.Fatalf("mutation %d: %+v", i+1, result)
		}
	}
	if v, _ := head(t, db.Pool, scope); v != 3 {
		t.Fatalf("head %d", v)
	}
	if last := lastMutation(t, db.Pool, client); last != 3 {
		t.Fatalf("last_mutation_id %d", last)
	}
}

func TestDuplicateMutationsAreSkipped(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	scope, client := uuid.NewString(), uuid.NewString()
	mustMutate(t, w, scope, client, 1, storetest.InsertNote(uuid.NewString(), scope, "a"))
	mustMutate(t, w, scope, client, 2, storetest.InsertNote(uuid.NewString(), scope, "b"))
	ran := false
	for _, id := range []uint64{2, 1} {
		result := mustMutate(t, w, scope, client, id, func(pgx.Tx) error { ran = true; return nil })
		if result.Status != storetest.Duplicate {
			t.Fatalf("mutation %d: %+v", id, result)
		}
	}
	if ran {
		t.Fatal("a duplicate mutation ran its body")
	}
	if v, _ := head(t, db.Pool, scope); v != 2 {
		t.Fatalf("head %d", v)
	}
	var notes int
	if err := db.Pool.QueryRow(context.Background(), "SELECT count(*) FROM note WHERE scope_id = $1", scope).Scan(&notes); err != nil {
		t.Fatal(err)
	}
	if notes != 2 {
		t.Fatalf("%d notes", notes)
	}
}

func TestMutationIDsMayJumpButNotGoBack(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	scope, client := uuid.NewString(), uuid.NewString()
	mustMutate(t, w, scope, client, 5, storetest.InsertNote(uuid.NewString(), scope, "a"))
	if result := mustMutate(t, w, scope, client, 4, storetest.InsertNote(uuid.NewString(), scope, "b")); result.Status != storetest.Duplicate {
		t.Fatalf("got %+v", result)
	}
	if last := lastMutation(t, db.Pool, client); last != 5 {
		t.Fatalf("last %d", last)
	}
}

func TestRejectionKeepsNoRowChanges(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	scope, client := uuid.NewString(), uuid.NewString()
	note := uuid.NewString()
	result := mustMutate(t, w, scope, client, 1, func(tx pgx.Tx) error {
		if err := storetest.InsertNote(note, scope, "doomed")(tx); err != nil {
			return err
		}
		return storetest.Rejection{Reason: "titles must be unique"}
	})
	if result != (storetest.Result{Status: storetest.Rejected, Version: 1}) {
		t.Fatalf("got %+v", result)
	}
	var notes int
	if err := db.Pool.QueryRow(context.Background(), "SELECT count(*) FROM note WHERE id = $1", note).Scan(&notes); err != nil {
		t.Fatal(err)
	}
	if notes != 0 {
		t.Fatal("the rejected mutation's row stayed")
	}
	log := readLog(t, db.Pool, scope)
	if len(log) != 1 || log[0].Kind != "rejection" || log[0].Seq != 0 || log[0].MutationID != 1 {
		t.Fatalf("log %+v", log)
	}
	if log[0].Data == nil || *log[0].Data != `{"reason": "titles must be unique"}` {
		t.Fatalf("rejection data %v", log[0].Data)
	}
	if last := lastMutation(t, db.Pool, client); last != 1 {
		t.Fatalf("a rejection must still ack the mutation, last %d", last)
	}
	if again := mustMutate(t, w, scope, client, 1, storetest.InsertNote(note, scope, "retry")); again.Status != storetest.Duplicate {
		t.Fatalf("a retried rejected mutation must be a duplicate, got %+v", again)
	}
}

func TestChangesAreCapturedInOrder(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	scope, client := uuid.NewString(), uuid.NewString()
	note, tag := uuid.NewString(), uuid.NewString()
	mustMutate(t, w, scope, client, 1, func(tx pgx.Tx) error {
		if err := storetest.InsertNote(note, scope, "first")(tx); err != nil {
			return err
		}
		if err := storetest.TagNote(tag, note, "home")(tx); err != nil {
			return err
		}
		return storetest.RenameNote(note, "second")(tx)
	})
	log := readLog(t, db.Pool, scope)
	if len(log) != 4 {
		t.Fatalf("log %+v", log)
	}
	if log[0].Kind != "commit" || log[0].Hash == nil {
		t.Fatalf("header %+v", log[0])
	}
	wantTables := []string{"note", "note_tag", "note"}
	for i, row := range log[1:] {
		if row.Kind != "patch" || row.Seq != i+1 || *row.Table != wantTables[i] || *row.Op != "upsert" || row.Hash != nil {
			t.Fatalf("patch %d: %+v", i, row)
		}
	}
	if *log[1].RowID != note || *log[2].RowID != tag {
		t.Fatalf("row ids %s %s", *log[1].RowID, *log[2].RowID)
	}
	if !strings.Contains(*log[3].Data, `"title": "second"`) {
		t.Fatalf("data %s", *log[3].Data)
	}
}

func TestExcludedColumnsNeverLeaveTheServer(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	scope := uuid.NewString()
	mustMutate(t, w, scope, uuid.NewString(), 1, storetest.InsertNote(uuid.NewString(), scope, "x"))
	for _, row := range readLog(t, db.Pool, scope) {
		if row.Data != nil && strings.Contains(*row.Data, "secret") {
			t.Fatalf("excluded column logged: %s", *row.Data)
		}
	}
}

func TestUnchangedUpdatesAreNotLogged(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	scope, client := uuid.NewString(), uuid.NewString()
	note := uuid.NewString()
	mustMutate(t, w, scope, client, 1, storetest.InsertNote(note, scope, "same"))
	mustMutate(t, w, scope, client, 2, storetest.RenameNote(note, "same"))
	log := readLog(t, db.Pool, scope)
	if len(log) != 3 || log[2].Version != 2 || log[2].Kind != "commit" {
		t.Fatalf("log %+v", log)
	}
}

func TestCascadeDeletesAreCaptured(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	scope, client := uuid.NewString(), uuid.NewString()
	note, tagA, tagB := uuid.NewString(), uuid.NewString(), uuid.NewString()
	mustMutate(t, w, scope, client, 1, storetest.InsertNote(note, scope, "x"))
	mustMutate(t, w, scope, client, 2, storetest.TagNote(tagA, note, "a"))
	mustMutate(t, w, scope, client, 3, storetest.TagNote(tagB, note, "b"))
	mustMutate(t, w, scope, client, 4, storetest.DeleteNote(note))
	deleted := map[string]string{}
	for _, row := range readLog(t, db.Pool, scope) {
		if row.Version != 4 || row.Kind != "patch" {
			continue
		}
		if *row.Op != "delete" || row.Data != nil {
			t.Fatalf("patch %+v", row)
		}
		deleted[*row.RowID] = *row.Table
	}
	want := map[string]string{note: "note", tagA: "note_tag", tagB: "note_tag"}
	if fmt.Sprint(deleted) != fmt.Sprint(want) {
		t.Fatalf("deleted %v, want %v", deleted, want)
	}
}

func TestHashChainMatchesAnIndependentComputation(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	ctx := context.Background()
	scope, client := uuid.NewString(), uuid.NewString()
	note := uuid.NewString()
	mustMutate(t, w, scope, client, 1, storetest.InsertNote(note, scope, "hash me"))
	mustMutate(t, w, scope, client, 2, func(pgx.Tx) error { return storetest.Rejection{Reason: "no"} })
	if _, err := w.ServerWrite(ctx, scope, storetest.TagNote(uuid.NewString(), note, "server")); err != nil {
		t.Fatal(err)
	}
	mustMutate(t, w, scope, client, 3, func(pgx.Tx) error { return nil })
	mustMutate(t, w, scope, client, 4, storetest.DeleteNote(note))

	log := readLog(t, db.Pool, scope)
	prev := []byte{}
	for i := 0; i < len(log); {
		header := log[i]
		var patches []logRow
		for i++; i < len(log) && log[i].Version == header.Version; i++ {
			patches = append(patches, log[i])
		}
		want := chainHash(prev, header, patches)
		if !bytes.Equal(header.Hash, want) {
			t.Fatalf("version %d: hash %x, independently %x", header.Version, header.Hash, want)
		}
		prev = want
	}
	version, headHash := head(t, db.Pool, scope)
	if version != 5 || !bytes.Equal(headHash, prev) {
		t.Fatalf("head %d %x, chain %x", version, headHash, prev)
	}
}

func chainHash(prev []byte, header logRow, patches []logRow) []byte {
	var body []byte
	if header.Kind == "rejection" {
		reason := strings.TrimSuffix(strings.TrimPrefix(*header.Data, `{"reason": "`), `"}`)
		sum := sha256.Sum256([]byte(reason))
		body = sum[:]
	} else {
		lines := make([]string, len(patches))
		for i, p := range patches {
			data := ""
			if p.Data != nil {
				data = *p.Data
			}
			lines[i] = fmt.Sprintf("%d|%s|%s|%s|%s", p.Seq, *p.Table, *p.RowID, *p.Op, data)
		}
		sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
		body = sum[:]
	}
	h := sha256.New()
	h.Write(prev)
	fmt.Fprintf(h, "%d|%s|%d|%s", header.Version, header.ClientID, header.MutationID, header.Kind)
	h.Write(body)
	return h.Sum(nil)
}

func TestWritesOutsideTheWritePathAreRefused(t *testing.T) {
	db := storetest.NewDatabase(t)
	ctx := context.Background()
	scope := uuid.NewString()
	_, err := db.Pool.Exec(ctx, "INSERT INTO note (id, scope_id, title) VALUES ($1, $2, 'sneaky')", uuid.NewString(), scope)
	if sqlState(err) != "55000" {
		t.Fatalf("insert: %v", err)
	}
	w := storetest.Writer{Pool: db.Pool}
	note := uuid.NewString()
	mustMutate(t, w, scope, uuid.NewString(), 1, storetest.InsertNote(note, scope, "legit"))
	for _, sql := range []string{"UPDATE note SET title = 'x' WHERE id = $1", "DELETE FROM note WHERE id = $1"} {
		if _, err := db.Pool.Exec(ctx, sql, note); sqlState(err) != "55000" {
			t.Fatalf("%s: %v", sql, err)
		}
	}
}

func TestWritesAfterTheMutationClosedAreRefused(t *testing.T) {
	db := storetest.NewDatabase(t)
	ctx := context.Background()
	scope := uuid.NewString()
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT store_begin($1, $2, 1)", scope, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SELECT store_end()"); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, "INSERT INTO note (id, scope_id, title) VALUES ($1, $2, 'late')", uuid.NewString(), scope)
	if sqlState(err) != "55000" {
		t.Fatalf("got %v", err)
	}
}

func TestBypassAllowsUnsyncedMigrations(t *testing.T) {
	db := storetest.NewDatabase(t)
	ctx := context.Background()
	scope := uuid.NewString()
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SET LOCAL store.bypass = on"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO note (id, scope_id, title) VALUES ($1, $2, 'backfill')", uuid.NewString(), scope); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if log := readLog(t, db.Pool, scope); len(log) != 0 {
		t.Fatalf("bypassed write was logged: %+v", log)
	}
}

func TestServerWritesAreLoggedUnderTheServerClient(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	ctx := context.Background()
	scope, client := uuid.NewString(), uuid.NewString()
	note := uuid.NewString()
	mustMutate(t, w, scope, client, 1, storetest.InsertNote(note, scope, "device"))
	version, err := w.ServerWrite(ctx, scope, storetest.RenameNote(note, "server"))
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("version %d", version)
	}
	log := readLog(t, db.Pool, scope)
	header := log[2]
	if header.Version != 2 || header.Kind != "commit" || header.ClientID != serverClient || header.MutationID != 2 {
		t.Fatalf("header %+v", header)
	}
	if patch := log[3]; patch.ClientID != serverClient || *patch.Op != "upsert" {
		t.Fatalf("patch %+v", patch)
	}
	if last := lastMutation(t, db.Pool, client); last != 1 {
		t.Fatalf("a server write moved a client's ack to %d", last)
	}
	var serverRows int
	if err := db.Pool.QueryRow(ctx, "SELECT count(*) FROM store_client WHERE client_id = $1", serverClient).Scan(&serverRows); err != nil {
		t.Fatal(err)
	}
	if serverRows != 0 {
		t.Fatal("the server client got a client row")
	}
}

func TestServerWritesCreateTheScope(t *testing.T) {
	db := storetest.NewDatabase(t)
	scope := uuid.NewString()
	version, err := storetest.Writer{Pool: db.Pool}.ServerWrite(context.Background(), scope, storetest.InsertNote(uuid.NewString(), scope, "import"))
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := head(t, db.Pool, scope); version != 1 || v != 1 {
		t.Fatalf("version %d head %d", version, v)
	}
}

func TestBeginRefusesInvalidCallers(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	ctx := context.Background()
	scope, other, client := uuid.NewString(), uuid.NewString(), uuid.NewString()
	mustMutate(t, w, scope, client, 1, func(pgx.Tx) error { return nil })
	noop := func(pgx.Tx) error { return nil }
	cases := map[string]struct {
		scope, client string
		mutation      uint64
		state         string
	}{
		"client of another scope": {other, client, 2, "42501"},
		"reserved server client":  {scope, serverClient, 1, "22023"},
		"mutation id zero":        {scope, uuid.NewString(), 0, "22023"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := w.Mutate(ctx, tc.scope, tc.client, tc.mutation, noop)
			if sqlState(err) != tc.state {
				t.Fatalf("got %v, want SQLSTATE %s", err, tc.state)
			}
		})
	}
}

func TestPokeNotifiesListeners(t *testing.T) {
	db := storetest.NewDatabase(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, db.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "LISTEN store_poke"); err != nil {
		t.Fatal(err)
	}
	scope := uuid.NewString()
	mustMutate(t, storetest.Writer{Pool: db.Pool}, scope, uuid.NewString(), 1, func(pgx.Tx) error { return nil })
	n, err := conn.WaitForNotification(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n.Payload != scope+":1" {
		t.Fatalf("payload %q", n.Payload)
	}
}

func TestConcurrentWritersKeepVersionsDense(t *testing.T) {
	db := storetest.NewDatabase(t)
	w := storetest.Writer{Pool: db.Pool}
	scope := uuid.NewString()
	const clients, perClient = 8, 10
	var wg sync.WaitGroup
	errs := make(chan error, clients)
	for range clients {
		wg.Go(func() {
			client := uuid.NewString()
			for m := uint64(1); m <= perClient; m++ {
				if _, err := w.Mutate(context.Background(), scope, client, m, storetest.InsertNote(uuid.NewString(), scope, "n")); err != nil {
					errs <- err
					return
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	expected := int64(1)
	for _, row := range readLog(t, db.Pool, scope) {
		if row.Seq != 0 {
			continue
		}
		if row.Version != expected {
			t.Fatalf("version %d, want %d", row.Version, expected)
		}
		expected++
	}
	if expected-1 != clients*perClient {
		t.Fatalf("%d versions", expected-1)
	}
}
