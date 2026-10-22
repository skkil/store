// Package writepath is the Go adapter for the store write path in
// modules/sql. It runs each device mutation in one transaction exactly as
// docs/protocol.md prescribes: store_begin, a savepoint, the mutation body,
// then store_end or (on a business-rule failure) a rollback to the savepoint
// and store_reject, and store_poke after commit.
package writepath

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Body runs one mutation's ordinary SQL on registered tables. Returning a
// Rejection rejects the mutation; any other error aborts the transaction so
// the device retries.
type Body func(ctx context.Context, m *Mutation) error

// ServerBody runs a write the product makes on its own.
type ServerBody func(ctx context.Context, tx pgx.Tx) error

// Writer applies mutations to the product's Postgres.
type Writer struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// New returns a Writer over pool. Poke failures are logged to logger, since
// pokes are only hints.
func New(pool *pgxpool.Pool, logger *slog.Logger) *Writer {
	return &Writer{pool: pool, logger: logger}
}

// Apply runs the device mutation (client, mutationID) in scope through body,
// at most once.
func (w *Writer) Apply(ctx context.Context, scope, client uuid.UUID, mutationID uint64, body Body) (Result, error) {
	var result Result
	err := pgx.BeginFunc(ctx, w.pool, func(tx pgx.Tx) error {
		var err error
		result, err = apply(ctx, tx, scope, client, mutationID, body)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	if result.Status != Duplicate {
		w.poke(ctx, scope, result.Version)
	}
	return result, nil
}

func apply(ctx context.Context, tx pgx.Tx, scope, client uuid.UUID, mutationID uint64, body Body) (Result, error) {
	duplicate, err := Begin(ctx, tx, scope, client, mutationID)
	if err != nil {
		return Result{}, err
	}
	if duplicate {
		version, err := RecordedVersion(ctx, tx, scope, client, mutationID)
		return Result{Status: Duplicate, Version: version}, err
	}
	if err := Savepoint(ctx, tx); err != nil {
		return Result{}, err
	}
	bodyErr := body(ctx, &Mutation{Tx: tx, Scope: scope, Client: client, ID: mutationID})
	if bodyErr == nil {
		version, err := End(ctx, tx)
		return Result{Status: Applied, Version: version}, err
	}
	rejection, ok := AsRejection(bodyErr)
	if !ok {
		return Result{}, bodyErr
	}
	if err := RollbackToSavepoint(ctx, tx); err != nil {
		return Result{}, err
	}
	version, err := RecordRejection(ctx, tx, rejection.Reason)
	return Result{Status: Rejected, Version: version, Reason: rejection.Reason}, err
}

// ApplyServer runs a write the product makes on its own (a REST endpoint, a
// job, an import). Devices pull it like any mutation. It returns the new version.
func (w *Writer) ApplyServer(ctx context.Context, scope uuid.UUID, body ServerBody) (uint64, error) {
	var version uint64
	err := pgx.BeginFunc(ctx, w.pool, func(tx pgx.Tx) error {
		if err := BeginServer(ctx, tx, scope); err != nil {
			return err
		}
		if err := body(ctx, tx); err != nil {
			return err
		}
		var err error
		version, err = End(ctx, tx)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("server write: %w", err)
	}
	w.poke(ctx, scope, version)
	return version, nil
}

func (w *Writer) poke(ctx context.Context, scope uuid.UUID, version uint64) {
	if err := Poke(ctx, w.pool, scope, version); err != nil {
		w.logger.Warn("store_poke failed", "scope", scope, "version", version, "error", err)
	}
}
