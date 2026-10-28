package suite

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skkil/store/modules/conformance/writepath"
)

// EpochBumper bumps a scope's epoch, as a restore, failover or shard move
// does (docs/protocol.md, Read path), and returns the new epoch.
type EpochBumper interface {
	Bump(ctx context.Context, scope string) (uint32, error)
}

// ErrNoScope means the scope has no store_scope row yet.
var ErrNoScope = errors.New("scope has no log yet")

// PostgresEpochs bumps epochs with store_bump_epoch in the product's database.
type PostgresEpochs struct {
	Pool *pgxpool.Pool
}

// Bump implements EpochBumper.
func (p PostgresEpochs) Bump(ctx context.Context, scope string) (uint32, error) {
	id, err := uuid.Parse(scope)
	if err != nil {
		return 0, fmt.Errorf("bump epoch: %w", err)
	}
	epoch, ok, err := writepath.BumpEpoch(ctx, p.Pool, id)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNoScope
	}
	return epoch, nil
}
