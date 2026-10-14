package pull

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	storev1 "github.com/skkil/store/modules/service/gen/storev1"
)

// initialEpoch is the epoch of a scope that has no store_scope row yet.
const initialEpoch = 1

// Log reads store_scope, store_client and store_log.
type Log struct {
	pool *pgxpool.Pool
}

// NewLog returns a Log reading through pool.
func NewLog(pool *pgxpool.Pool) *Log {
	return &Log{pool: pool}
}

// View runs fn against one consistent, read-only snapshot of the database.
func (l *Log) View(ctx context.Context, fn func(*View) error) error {
	return pgx.BeginTxFunc(ctx, l.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		return fn(&View{q: tx})
	})
}

// Head returns the newest committed state of scope.
func (l *Log) Head(ctx context.Context, scope string) (Head, error) {
	return (&View{q: l.pool}).Head(ctx, scope)
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// View reads from one snapshot.
type View struct {
	q querier
}

// Head returns the newest committed state of scope.
func (v *View) Head(ctx context.Context, scope string) (Head, error) {
	var epoch int32
	var version int64
	var hash []byte
	err := v.q.QueryRow(ctx,
		"SELECT epoch, head_version, head_hash FROM store_scope WHERE scope_id = $1", scope,
	).Scan(&epoch, &version, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Head{Epoch: initialEpoch, Hash: []byte{}}, nil
	}
	if err != nil {
		return Head{}, fmt.Errorf("read head: %w", err)
	}
	return Head{Epoch: uint32(epoch), Version: uint64(version), Hash: hash}, nil
}

// LastMutationID returns the highest mutation of client in scope the server
// has processed, or 0.
func (v *View) LastMutationID(ctx context.Context, scope, client string) (uint64, error) {
	var last int64
	err := v.q.QueryRow(ctx,
		"SELECT last_mutation_id FROM store_client WHERE client_id = $1 AND scope_id = $2", client, scope,
	).Scan(&last)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read last mutation: %w", err)
	}
	return uint64(last), nil
}

// HashAt returns the chain hash recorded at version, or nil when the log
// holds no such version.
func (v *View) HashAt(ctx context.Context, scope string, version uint64) ([]byte, error) {
	var hash []byte
	err := v.q.QueryRow(ctx,
		"SELECT hash FROM store_log WHERE scope_id = $1 AND version = $2 AND seq = 0", scope, int64(version),
	).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read hash: %w", err)
	}
	return hash, nil
}

// Versions returns every version in (after, through], each with its changes
// in capture order or its rejection.
func (v *View) Versions(ctx context.Context, scope string, after, through uint64) ([]*storev1.Version, error) {
	rows, err := v.q.Query(ctx, `
		SELECT version, seq, kind, table_name, row_id, op, data::text, client_id::text, mutation_id, hash, committed_at
		  FROM store_log
		 WHERE scope_id = $1 AND version > $2 AND version <= $3
		 ORDER BY version, seq`,
		scope, int64(after), int64(through))
	if err != nil {
		return nil, fmt.Errorf("read versions: %w", err)
	}
	defer rows.Close()
	var assembler versionAssembler
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.version, &e.seq, &e.kind, &e.table, &e.rowID, &e.op, &e.data, &e.clientID, &e.mutationID, &e.hash, &e.committedAt); err != nil {
			return nil, fmt.Errorf("read versions: %w", err)
		}
		if err := assembler.add(e); err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read versions: %w", err)
	}
	return assembler.versions, nil
}
