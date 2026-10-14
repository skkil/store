package poke

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PollBus finds advanced scopes by polling store_scope.updated_at. It is the
// fallback for notifications the listener missed.
//
// updated_at is the writing transaction's start time, so a write is seen as
// long as it commits within Lookback of starting.
type PollBus struct {
	pool     *pgxpool.Pool
	interval time.Duration
	lookback time.Duration
	logger   *slog.Logger
}

// DefaultLookback is how far before the previous poll each poll looks.
const DefaultLookback = time.Minute

// NewPollBus returns a PollBus polling every interval.
func NewPollBus(pool *pgxpool.Pool, interval, lookback time.Duration, logger *slog.Logger) *PollBus {
	return &PollBus{pool: pool, interval: interval, lookback: lookback, logger: logger}
}

type seenScope struct {
	version   uint64
	updatedAt time.Time
}

// Run implements PokeBus.
func (b *PollBus) Run(ctx context.Context, deliver func(Notification)) error {
	seen := map[string]seenScope{}
	var since time.Time
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()
	for {
		next, err := b.poll(ctx, since, seen, deliver)
		if err == nil {
			since = next
		} else if ctx.Err() == nil {
			b.logger.Warn("poke poll failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (b *PollBus) poll(ctx context.Context, since time.Time, seen map[string]seenScope, deliver func(Notification)) (time.Time, error) {
	var now time.Time
	if err := b.pool.QueryRow(ctx, "SELECT now()").Scan(&now); err != nil {
		return since, err
	}
	if since.IsZero() {
		since = now.Add(-b.lookback)
	}
	rows, err := b.pool.Query(ctx,
		"SELECT scope_id::text, head_version, updated_at FROM store_scope WHERE updated_at > $1", since)
	if err != nil {
		return since, err
	}
	defer rows.Close()
	var changed []Notification
	for rows.Next() {
		var scope string
		var version int64
		var updatedAt time.Time
		if err := rows.Scan(&scope, &version, &updatedAt); err != nil {
			return since, err
		}
		if prev, ok := seen[scope]; ok && prev.version >= uint64(version) {
			continue
		}
		seen[scope] = seenScope{version: uint64(version), updatedAt: updatedAt}
		changed = append(changed, Notification{Scope: scope, Version: uint64(version)})
	}
	if err := rows.Err(); err != nil {
		return since, err
	}
	next := now.Add(-b.lookback)
	for scope, s := range seen {
		if s.updatedAt.Before(next) {
			delete(seen, scope)
		}
	}
	for _, n := range changed {
		deliver(n)
	}
	return next, nil
}
