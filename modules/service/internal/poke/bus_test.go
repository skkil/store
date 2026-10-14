package poke_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/skkil/store/modules/service/internal/poke"
	"github.com/skkil/store/modules/service/internal/storetest"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func runBus(t *testing.T, bus poke.PokeBus) <-chan poke.Notification {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan poke.Notification, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = bus.Run(ctx, func(n poke.Notification) { out <- n })
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return out
}

func await(t *testing.T, ch <-chan poke.Notification, want poke.Notification) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case n := <-ch:
			if n == want {
				return
			}
		case <-deadline:
			t.Fatalf("no notification %+v", want)
		}
	}
}

func waitListening(t *testing.T, bus *poke.ListenBus) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !bus.Listening() {
		if time.Now().After(deadline) {
			t.Fatal("listener never connected")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func noop(pgx.Tx) error { return nil }

func TestListenBusDeliversStorePoke(t *testing.T) {
	db := storetest.NewDatabase(t)
	bus := poke.NewListenBus(db.URL, quiet)
	got := runBus(t, bus)
	waitListening(t, bus)
	scope := uuid.NewString()
	if _, err := (storetest.Writer{Pool: db.Pool}).Mutate(context.Background(), scope, uuid.NewString(), 1, noop); err != nil {
		t.Fatal(err)
	}
	await(t, got, poke.Notification{Scope: scope, Version: 1})
}

func TestListenBusIgnoresMalformedPayloads(t *testing.T) {
	db := storetest.NewDatabase(t)
	bus := poke.NewListenBus(db.URL, quiet)
	got := runBus(t, bus)
	waitListening(t, bus)
	scope := uuid.NewString()
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, "SELECT pg_notify('store_poke', 'garbage')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, "SELECT store_poke($1, 8)", scope); err != nil {
		t.Fatal(err)
	}
	select {
	case n := <-got:
		if n != (poke.Notification{Scope: scope, Version: 8}) {
			t.Fatalf("got %+v", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no notification")
	}
}

func TestListenBusReconnects(t *testing.T) {
	db := storetest.NewDatabase(t)
	bus := poke.NewListenBus(db.URL, quiet)
	got := runBus(t, bus)
	waitListening(t, bus)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx,
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE query LIKE 'LISTEN%' AND datname = current_database()"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for bus.Listening() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	waitListening(t, bus)
	scope := uuid.NewString()
	if _, err := db.Pool.Exec(ctx, "SELECT store_poke($1, 3)", scope); err != nil {
		t.Fatal(err)
	}
	await(t, got, poke.Notification{Scope: scope, Version: 3})
}

func TestPollBusFindsWritesWithoutPokes(t *testing.T) {
	db := storetest.NewDatabase(t)
	got := runBus(t, poke.NewPollBus(db.Pool, 20*time.Millisecond, poke.DefaultLookback, quiet))
	w := storetest.Writer{Pool: db.Pool, SkipPoke: true}
	scope, client := uuid.NewString(), uuid.NewString()
	ctx := context.Background()
	if _, err := w.Mutate(ctx, scope, client, 1, noop); err != nil {
		t.Fatal(err)
	}
	await(t, got, poke.Notification{Scope: scope, Version: 1})
	if _, err := w.Mutate(ctx, scope, client, 2, noop); err != nil {
		t.Fatal(err)
	}
	await(t, got, poke.Notification{Scope: scope, Version: 2})
	select {
	case n := <-got:
		t.Fatalf("duplicate notification %+v", n)
	case <-time.After(100 * time.Millisecond):
	}
}
