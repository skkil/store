package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/skkil/store/modules/service/internal/storetest"
)

func TestPokesSendTheHeadThenEachNewVersion(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	inst := start(t, baseConfig(t, db, issuer))
	d := newDevice(t, issuer)
	w := storetest.Writer{Pool: db.Pool}
	mutate(t, w, d, 1)
	pokes := openPokes(t, inst.client(), d)
	if v := pokes.next(t); v != 1 {
		t.Fatalf("first poke %d, want the head 1", v)
	}
	waitForListener(t, db)
	mutate(t, w, d, 2)
	pokes.await(t, 2)
	mutate(t, w, d, 3)
	pokes.await(t, 3)
}

func TestPokesStayInTheirScope(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	inst := start(t, baseConfig(t, db, issuer))
	mine, theirs := newDevice(t, issuer), newDevice(t, issuer)
	pokes := openPokes(t, inst.client(), mine)
	if v := pokes.next(t); v != 0 {
		t.Fatalf("head %d", v)
	}
	waitForListener(t, db)
	w := storetest.Writer{Pool: db.Pool}
	mutate(t, w, theirs, 1)
	mutate(t, w, mine, 1)
	if v := pokes.next(t); v != 1 {
		t.Fatalf("got %d", v)
	}
	select {
	case v := <-pokes.versions:
		t.Fatalf("unexpected poke %d", v)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestPokesFallBackToPolling(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	cfg := baseConfig(t, db, issuer)
	cfg.PollInterval = 50 * time.Millisecond
	inst := start(t, cfg)
	d := newDevice(t, issuer)
	pokes := openPokes(t, inst.client(), d)
	pokes.next(t)
	silent := storetest.Writer{Pool: db.Pool, SkipPoke: true}
	mutate(t, silent, d, 1)
	pokes.await(t, 1)
	mutate(t, silent, d, 2)
	pokes.await(t, 2)
}

func TestPokesKeepTheStreamAlive(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	cfg := baseConfig(t, db, issuer)
	cfg.PokeKeepAlive = 50 * time.Millisecond
	inst := start(t, cfg)
	d := newDevice(t, issuer)
	mutate(t, storetest.Writer{Pool: db.Pool}, d, 1)
	pokes := openPokes(t, inst.client(), d)
	for range 3 {
		if v := pokes.next(t); v != 1 {
			t.Fatalf("keep-alive carried %d, want 1", v)
		}
	}
}

func TestTwoInstancesServeTheSameDatabase(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	a := start(t, baseConfig(t, db, issuer))
	b := start(t, baseConfig(t, db, issuer))
	d := newDevice(t, issuer)
	pokesA := openPokes(t, a.client(), d)
	pokesB := openPokes(t, b.h2cClient(), d)
	pokesA.next(t)
	pokesB.next(t)
	waitForListeners(t, db, 2)

	w := storetest.Writer{Pool: db.Pool}
	for i := uint64(1); i <= 3; i++ {
		mutate(t, w, d, i)
	}
	pokesA.await(t, 3)
	pokesB.await(t, 3)

	fromA, err := a.client().Pull(context.Background(), pullRequest(d, nil))
	if err != nil {
		t.Fatal(err)
	}
	versions := fromA.Msg.GetEntries().GetVersions()
	if len(versions) != 3 {
		t.Fatalf("instance a: %v", fromA.Msg)
	}
	fromB, err := b.client().Pull(context.Background(), pullRequest(d, cursorOf(versions[1], 1)))
	if err != nil {
		t.Fatal(err)
	}
	rest := fromB.Msg.GetEntries().GetVersions()
	if len(rest) != 1 || rest[0].GetVersion() != 3 || string(rest[0].GetHash()) != string(versions[2].GetHash()) {
		t.Fatalf("instance b continued with %v", fromB.Msg)
	}
}

func TestShutdownEndsStreams(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	inst := start(t, baseConfig(t, db, issuer))
	d := newDevice(t, issuer)
	pokes := openPokes(t, inst.client(), d)
	pokes.next(t)
	if err := inst.shutdown(t); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	select {
	case _, ok := <-pokes.versions:
		if ok {
			t.Fatal("stream kept going")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not end")
	}
}

func waitForListener(t *testing.T, db *storetest.Database) {
	t.Helper()
	waitForListeners(t, db, 1)
}

func waitForListeners(t *testing.T, db *storetest.Database, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var listeners int
		err := db.Pool.QueryRow(context.Background(),
			"SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND query LIKE 'LISTEN%'").Scan(&listeners)
		if err != nil {
			t.Fatal(err)
		}
		if listeners >= n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d of %d listeners connected", listeners, n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
