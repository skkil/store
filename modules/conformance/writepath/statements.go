package writepath

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// SavepointName is the savepoint taken right after store_begin; a rejection
// rolls back to it.
const SavepointName = "store_mutation"

// Begin calls store_begin and reports whether (client, mutationID) was
// already processed.
func Begin(ctx context.Context, tx pgx.Tx, scope, client uuid.UUID, mutationID uint64) (duplicate bool, err error) {
	var outcome string
	if err := tx.QueryRow(ctx, "SELECT store_begin($1, $2, $3)", scope, client, int64(mutationID)).Scan(&outcome); err != nil {
		return false, fmt.Errorf("store_begin: %w", err)
	}
	switch outcome {
	case "begun":
		return false, nil
	case "duplicate":
		return true, nil
	default:
		return false, fmt.Errorf("store_begin: unknown outcome %q", outcome)
	}
}

// BeginServer calls store_begin_server for a write the product makes on its own.
func BeginServer(ctx context.Context, tx pgx.Tx, scope uuid.UUID) error {
	if _, err := tx.Exec(ctx, "SELECT store_begin_server($1)", scope); err != nil {
		return fmt.Errorf("store_begin_server: %w", err)
	}
	return nil
}

// Savepoint takes SavepointName.
func Savepoint(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, "SAVEPOINT "+SavepointName); err != nil {
		return fmt.Errorf("savepoint: %w", err)
	}
	return nil
}

// RollbackToSavepoint undoes every row change made after Savepoint.
func RollbackToSavepoint(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+SavepointName); err != nil {
		return fmt.Errorf("rollback to savepoint: %w", err)
	}
	return nil
}

// End calls store_end and returns the new version.
func End(ctx context.Context, tx pgx.Tx) (uint64, error) {
	var version int64
	if err := tx.QueryRow(ctx, "SELECT store_end()").Scan(&version); err != nil {
		return 0, fmt.Errorf("store_end: %w", err)
	}
	return uint64(version), nil
}

// RecordRejection calls store_reject and returns the version that records it.
func RecordRejection(ctx context.Context, tx pgx.Tx, reason string) (uint64, error) {
	var version int64
	if err := tx.QueryRow(ctx, "SELECT store_reject($1)", reason).Scan(&version); err != nil {
		return 0, fmt.Errorf("store_reject: %w", err)
	}
	return uint64(version), nil
}

// RecordedVersion returns the version that recorded an already processed
// mutation, or the scope head when the log no longer holds it.
func RecordedVersion(ctx context.Context, tx pgx.Tx, scope, client uuid.UUID, mutationID uint64) (uint64, error) {
	var version int64
	err := tx.QueryRow(ctx, `
		SELECT coalesce(
		  (SELECT version FROM store_log WHERE scope_id = $1 AND client_id = $2 AND mutation_id = $3 AND seq = 0),
		  (SELECT head_version FROM store_scope WHERE scope_id = $1),
		  0)`,
		scope, client, int64(mutationID)).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("recorded version: %w", err)
	}
	return uint64(version), nil
}

// Execer runs a statement on its own connection or transaction.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Poke calls store_poke. Call it after the mutation transaction commits,
// never inside it.
func Poke(ctx context.Context, db Execer, scope uuid.UUID, version uint64) error {
	if _, err := db.Exec(ctx, "SELECT store_poke($1, $2)", scope, int64(version)); err != nil {
		return fmt.Errorf("store_poke: %w", err)
	}
	return nil
}
