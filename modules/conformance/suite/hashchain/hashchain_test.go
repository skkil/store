package hashchain_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	storev1 "github.com/skkil/store/modules/conformance/gen/storev1"
	"github.com/skkil/store/modules/conformance/internal/stacktest"
	"github.com/skkil/store/modules/conformance/suite/hashchain"
	"github.com/skkil/store/modules/conformance/writepath"
)

func TestNextMatchesStoreCloseInPostgres(t *testing.T) {
	db := stacktest.NewDatabase(t)
	ctx := context.Background()
	w := writepath.New(db.Pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	scope, client := uuid.New(), uuid.New()
	todo := writepath.DeriveID(client, 1, 0)
	bodies := []writepath.Body{
		func(ctx context.Context, m *writepath.Mutation) error {
			if _, err := m.Tx.Exec(ctx, `INSERT INTO todo (id, scope_id, title) VALUES ($1, $2, 'quote " 한글 \ 🌱')`, m.NextID(), m.Scope); err != nil {
				return err
			}
			_, err := m.Tx.Exec(ctx, "INSERT INTO todo_item (id, todo_id, text) VALUES ($1, $2, 'x')", m.NextID(), todo)
			return err
		},
		func(context.Context, *writepath.Mutation) error { return writepath.Reject("nope, ünïcode") },
		func(context.Context, *writepath.Mutation) error { return nil },
		func(ctx context.Context, m *writepath.Mutation) error {
			_, err := m.Tx.Exec(ctx, "DELETE FROM todo WHERE id = $1", todo)
			return err
		},
	}
	for i, body := range bodies {
		if _, err := w.Apply(ctx, scope, client, uint64(i+1), body); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.ApplyServer(ctx, scope, func(context.Context, pgx.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	var prev []byte
	for _, v := range readLog(t, db, scope) {
		got := hashchain.Next(prev, v)
		if !bytes.Equal(got, v.GetHash()) {
			t.Fatalf("v%d: recomputed %x, store_close wrote %x", v.GetVersion(), got, v.GetHash())
		}
		prev = got
	}
}

func TestNextDependsOnEveryField(t *testing.T) {
	base := &storev1.Version{Version: 3, ClientId: uuid.NewString(), MutationId: 2, Changes: []*storev1.Change{
		{Table: "todo", RowId: "a", Op: storev1.Change_OP_UPSERT, Data: []byte(`{"title": "x"}`)},
	}}
	prev := []byte{1, 2, 3}
	want := hashchain.Next(prev, base)
	variants := map[string]func(v *storev1.Version){
		"version":   func(v *storev1.Version) { v.Version++ },
		"client":    func(v *storev1.Version) { v.ClientId = uuid.NewString() },
		"mutation":  func(v *storev1.Version) { v.MutationId++ },
		"data":      func(v *storev1.Version) { v.Changes[0].Data = []byte(`{"title": "y"}`) },
		"op":        func(v *storev1.Version) { v.Changes[0].Op = storev1.Change_OP_DELETE },
		"dropped":   func(v *storev1.Version) { v.Changes = nil },
		"rejection": func(v *storev1.Version) { v.Changes, v.Rejection = nil, &storev1.Rejection{Reason: ""} },
	}
	for name, mutate := range variants {
		v := cloneVersion(base)
		mutate(v)
		if bytes.Equal(hashchain.Next(prev, v), want) {
			t.Errorf("changing the %s left the hash unchanged", name)
		}
	}
	if bytes.Equal(hashchain.Next(nil, base), want) {
		t.Error("changing the previous hash left the hash unchanged")
	}
}

func cloneVersion(v *storev1.Version) *storev1.Version {
	out := &storev1.Version{Version: v.Version, ClientId: v.ClientId, MutationId: v.MutationId}
	for _, c := range v.Changes {
		out.Changes = append(out.Changes, &storev1.Change{Table: c.Table, RowId: c.RowId, Op: c.Op, Data: c.Data})
	}
	return out
}

func readLog(t *testing.T, db *stacktest.Database, scope uuid.UUID) []*storev1.Version {
	t.Helper()
	rows, err := db.Pool.Query(context.Background(), `
		SELECT version, seq, kind, coalesce(table_name, ''), coalesce(row_id, ''), coalesce(op, ''),
		       coalesce(data::text, ''), client_id::text, mutation_id, hash
		  FROM store_log WHERE scope_id = $1 ORDER BY version, seq`, scope)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var log []*storev1.Version
	for rows.Next() {
		var version, mutation int64
		var seq int32
		var kind, table, rowID, op, data, client string
		var hash []byte
		if err := rows.Scan(&version, &seq, &kind, &table, &rowID, &op, &data, &client, &mutation, &hash); err != nil {
			t.Fatal(err)
		}
		switch kind {
		case "patch":
			c := &storev1.Change{Table: table, RowId: rowID, Op: storev1.Change_OP_UPSERT, Data: []byte(data)}
			if op == "delete" {
				c.Op, c.Data = storev1.Change_OP_DELETE, nil
			}
			last := log[len(log)-1]
			last.Changes = append(last.Changes, c)
		default:
			v := &storev1.Version{Version: uint64(version), ClientId: client, MutationId: uint64(mutation), Hash: hash}
			if kind == "rejection" {
				var reason string
				if err := db.Pool.QueryRow(context.Background(), "SELECT $1::jsonb->>'reason'", data).Scan(&reason); err != nil {
					t.Fatal(err)
				}
				v.Rejection = &storev1.Rejection{MutationId: v.MutationId, Reason: reason}
			}
			log = append(log, v)
		}
	}
	if len(log) != 5 {
		t.Fatalf("read %d versions, want 5", len(log))
	}
	return log
}
