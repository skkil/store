package storetest

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Status is the outcome of one pushed mutation.
type Status string

// The outcomes a product server reports for a mutation.
const (
	Applied   Status = "applied"
	Duplicate Status = "duplicate"
	Rejected  Status = "rejected"
)

// Result is what the write path recorded for a mutation.
type Result struct {
	Status  Status
	Version uint64
}

// Rejection is returned by a mutation body to reject it with a reason.
type Rejection struct {
	Reason string
}

func (r Rejection) Error() string { return "rejected: " + r.Reason }

// Writer runs mutations through the store write path exactly as
// docs/protocol.md prescribes for product servers.
type Writer struct {
	Pool *pgxpool.Pool
	// SkipPoke leaves out store_poke after commit, as if the poke were lost.
	SkipPoke bool
}

// Mutate applies one device mutation. A body returning Rejection rolls back
// to the savepoint and records the rejection; any other error aborts.
func (w Writer) Mutate(ctx context.Context, scope, client string, mutationID uint64, body func(pgx.Tx) error) (Result, error) {
	tx, err := w.Pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var begun string
	if err := tx.QueryRow(ctx, "SELECT store_begin($1, $2, $3)", scope, client, int64(mutationID)).Scan(&begun); err != nil {
		return Result{}, err
	}
	if begun == "duplicate" {
		return Result{Status: Duplicate}, tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, "SAVEPOINT store_mutation"); err != nil {
		return Result{}, err
	}
	result := Result{Status: Applied}
	query := "SELECT store_end()"
	var rejection Rejection
	if err := body(tx); errors.As(err, &rejection) {
		if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT store_mutation"); err != nil {
			return Result{}, err
		}
		result.Status = Rejected
		query = "SELECT store_reject($1)"
	} else if err != nil {
		return Result{}, err
	}
	var version int64
	args := []any{}
	if result.Status == Rejected {
		args = append(args, rejection.Reason)
	}
	if err := tx.QueryRow(ctx, query, args...).Scan(&version); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	result.Version = uint64(version)
	return result, w.poke(ctx, scope, result.Version)
}

// ServerWrite applies a write the product makes on its own and returns the
// new version.
func (w Writer) ServerWrite(ctx context.Context, scope string, body func(pgx.Tx) error) (uint64, error) {
	tx, err := w.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT store_begin_server($1)", scope); err != nil {
		return 0, err
	}
	if err := body(tx); err != nil {
		return 0, err
	}
	var version int64
	if err := tx.QueryRow(ctx, "SELECT store_end()").Scan(&version); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return uint64(version), w.poke(ctx, scope, uint64(version))
}

func (w Writer) poke(ctx context.Context, scope string, version uint64) error {
	if w.SkipPoke {
		return nil
	}
	if _, err := w.Pool.Exec(ctx, "SELECT store_poke($1, $2)", scope, int64(version)); err != nil {
		return fmt.Errorf("poke: %w", err)
	}
	return nil
}
