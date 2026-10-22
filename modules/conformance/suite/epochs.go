package suite

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EpochBumper bumps a scope's epoch, as a restore, failover or shard move
// does (docs/protocol.md, Read path), and returns the new epoch.
type EpochBumper interface {
	Bump(ctx context.Context, scope string) (uint32, error)
}

// ErrNoScope means the scope has no store_scope row yet.
var ErrNoScope = errors.New("scope has no log yet")

// PostgresEpochs bumps epochs directly in the product's database.
type PostgresEpochs struct {
	Pool *pgxpool.Pool
}

// Bump implements EpochBumper.
func (p PostgresEpochs) Bump(ctx context.Context, scope string) (uint32, error) {
	var epoch int32
	err := p.Pool.QueryRow(ctx,
		"UPDATE store_scope SET epoch = epoch + 1, updated_at = clock_timestamp() WHERE scope_id = $1 RETURNING epoch",
		scope).Scan(&epoch)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoScope
	}
	if err != nil {
		return 0, fmt.Errorf("bump epoch: %w", err)
	}
	return uint32(epoch), nil
}
