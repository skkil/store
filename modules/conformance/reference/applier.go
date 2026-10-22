package reference

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skkil/store/modules/conformance/writepath"
)

// Applier runs one device mutation through a write path.
type Applier interface {
	Apply(ctx context.Context, scope, client uuid.UUID, mutationID uint64, body writepath.Body) (writepath.Result, error)
}

// NewApplier returns the write path for fault: the plain writepath.Writer
// for FaultNone, a deliberately broken one otherwise.
func NewApplier(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger, fault Fault) (Applier, error) {
	if fault == FaultNone {
		return writepath.New(pool, logger), nil
	}
	return newFaultyApplier(ctx, pool, fault)
}
