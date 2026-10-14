package poke_test

import (
	"testing"
	"time"

	"github.com/skkil/store/modules/service/internal/metrics"
	"github.com/skkil/store/modules/service/internal/poke"
)

func receive(t *testing.T, s *poke.Subscription) uint64 {
	t.Helper()
	select {
	case v, ok := <-s.Pokes():
		if !ok {
			t.Fatal("subscription closed")
		}
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("no poke")
		return 0
	}
}

func assertNothing(t *testing.T, s *poke.Subscription) {
	t.Helper()
	select {
	case v := <-s.Pokes():
		t.Fatalf("unexpected poke %d", v)
	case <-time.After(20 * time.Millisecond):
	}
}

func subscribe(t *testing.T, hub *poke.Hub, scope string) *poke.Subscription {
	t.Helper()
	s, ok := hub.Subscribe(scope)
	if !ok {
		t.Fatal("hub refused a subscription")
	}
	return s
}

func TestHubDeliversToTheScopeOnly(t *testing.T) {
	hub := poke.NewHub(metrics.New())
	a1, a2, b := subscribe(t, hub, "a"), subscribe(t, hub, "a"), subscribe(t, hub, "b")
	hub.Deliver(poke.Notification{Scope: "a", Version: 3})
	if receive(t, a1) != 3 || receive(t, a2) != 3 {
		t.Fatal("wrong version")
	}
	assertNothing(t, b)
}

func TestHubCoalescesUnreadPokes(t *testing.T) {
	hub := poke.NewHub(metrics.New())
	s := subscribe(t, hub, "a")
	for _, v := range []uint64{4, 9, 7} {
		hub.Deliver(poke.Notification{Scope: "a", Version: v})
	}
	if v := receive(t, s); v != 9 {
		t.Fatalf("got %d, want 9", v)
	}
	assertNothing(t, s)
}

func TestSubscriptionCloseStopsDelivery(t *testing.T) {
	hub := poke.NewHub(metrics.New())
	s := subscribe(t, hub, "a")
	s.Close()
	s.Close()
	if hub.Subscribers("a") != 0 {
		t.Fatal("subscription still registered")
	}
	hub.Deliver(poke.Notification{Scope: "a", Version: 1})
	if _, ok := <-s.Pokes(); ok {
		t.Fatal("closed subscription received a poke")
	}
}

func TestHubCloseEndsSubscriptions(t *testing.T) {
	hub := poke.NewHub(metrics.New())
	s := subscribe(t, hub, "a")
	hub.Close()
	if _, ok := <-s.Pokes(); ok {
		t.Fatal("subscription still open")
	}
	s.Close()
	if _, ok := hub.Subscribe("a"); ok {
		t.Fatal("closed hub accepted a subscription")
	}
}
