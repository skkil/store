package reference

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skkil/store/modules/conformance/writepath"
)

const versionSequence = "reference_version"

var errDropped = errors.New("rejection dropped")

type faultyApplier struct {
	pool  *pgxpool.Pool
	fault Fault
}

func newFaultyApplier(ctx context.Context, pool *pgxpool.Pool, fault Fault) (*faultyApplier, error) {
	if fault == FaultSequenceVersions {
		if _, err := pool.Exec(ctx, "CREATE SEQUENCE IF NOT EXISTS "+versionSequence); err != nil {
			return nil, fmt.Errorf("version sequence: %w", err)
		}
	}
	return &faultyApplier{pool: pool, fault: fault}, nil
}

func (a *faultyApplier) Apply(ctx context.Context, scope, client uuid.UUID, mutationID uint64, body writepath.Body) (writepath.Result, error) {
	var result writepath.Result
	err := pgx.BeginFunc(ctx, a.pool, func(tx pgx.Tx) error {
		var err error
		result, err = a.apply(ctx, tx, scope, client, mutationID, body)
		return err
	})
	if errors.Is(err, errDropped) {
		return result, nil
	}
	if err != nil {
		return writepath.Result{}, err
	}
	if result.Status != writepath.Duplicate {
		_ = writepath.Poke(ctx, a.pool, scope, result.Version)
	}
	return result, nil
}

func (a *faultyApplier) apply(ctx context.Context, tx pgx.Tx, scope, client uuid.UUID, mutationID uint64, body writepath.Body) (writepath.Result, error) {
	var drawn int64
	if a.fault == FaultSequenceVersions {
		if err := tx.QueryRow(ctx, "SELECT nextval($1)", versionSequence).Scan(&drawn); err != nil {
			return writepath.Result{}, err
		}
	}
	duplicate, err := writepath.Begin(ctx, tx, scope, client, mutationID)
	if err != nil {
		return writepath.Result{}, err
	}
	switch {
	case duplicate && a.fault == FaultIgnoreDuplicates:
		if err := writepath.BeginServer(ctx, tx, scope); err != nil {
			return writepath.Result{}, err
		}
	case duplicate:
		version, err := writepath.RecordedVersion(ctx, tx, scope, client, mutationID)
		return writepath.Result{Status: writepath.Duplicate, Version: version}, err
	case a.fault == FaultSequenceVersions:
		if _, err := tx.Exec(ctx, "SELECT set_config('store.version', $1, true)", strconv.FormatInt(drawn, 10)); err != nil {
			return writepath.Result{}, err
		}
	}
	if err := writepath.Savepoint(ctx, tx); err != nil {
		return writepath.Result{}, err
	}
	bodyErr := body(ctx, &writepath.Mutation{Tx: tx, Scope: scope, Client: client, ID: mutationID})
	if bodyErr == nil {
		version, err := writepath.End(ctx, tx)
		return writepath.Result{Status: writepath.Applied, Version: version}, err
	}
	rejection, ok := writepath.AsRejection(bodyErr)
	if !ok {
		return writepath.Result{}, bodyErr
	}
	if a.fault == FaultDropRejections {
		return writepath.Result{Status: writepath.Rejected, Reason: rejection.Reason}, errDropped
	}
	if err := writepath.RollbackToSavepoint(ctx, tx); err != nil {
		return writepath.Result{}, err
	}
	version, err := writepath.RecordRejection(ctx, tx, rejection.Reason)
	return writepath.Result{Status: writepath.Rejected, Version: version, Reason: rejection.Reason}, err
}
