package poke

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
)

// Channel is the Postgres notification channel store_poke() writes to.
const Channel = "store_poke"

// ListenBus receives store_poke notifications over a dedicated Postgres
// connection, reconnecting with backoff when it drops.
type ListenBus struct {
	connString string
	logger     *slog.Logger
	listening  atomic.Bool
}

// NewListenBus returns a ListenBus connecting with connString.
func NewListenBus(connString string, logger *slog.Logger) *ListenBus {
	return &ListenBus{connString: connString, logger: logger}
}

// Listening reports whether the bus currently holds a LISTEN connection.
func (b *ListenBus) Listening() bool {
	return b.listening.Load()
}

// Run implements PokeBus.
func (b *ListenBus) Run(ctx context.Context, deliver func(Notification)) error {
	backoff := newBackoff(100*time.Millisecond, 10*time.Second)
	for {
		err := b.listen(ctx, deliver, backoff.reset)
		b.listening.Store(false)
		if ctx.Err() != nil {
			return nil
		}
		wait := backoff.next()
		b.logger.Warn("poke listener disconnected", "error", err, "retry_in", wait)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
	}
}

func (b *ListenBus) listen(ctx context.Context, deliver func(Notification), connected func()) error {
	conn, err := pgx.Connect(ctx, b.connString)
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = conn.Close(closeCtx)
	}()
	if _, err := conn.Exec(ctx, "LISTEN "+Channel); err != nil {
		return err
	}
	b.listening.Store(true)
	connected()
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		parsed, err := ParseNotification(n.Payload)
		if err != nil {
			b.logger.Warn("ignoring malformed poke", "error", err)
			continue
		}
		deliver(parsed)
	}
}
