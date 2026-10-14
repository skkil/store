package poke

import (
	"sync"

	"github.com/skkil/store/modules/service/internal/metrics"
)

// Hub fans notifications out to the streams subscribed to each scope.
type Hub struct {
	mu      sync.Mutex
	scopes  map[string]map[*Subscription]struct{}
	closed  bool
	metrics *metrics.Metrics
}

// NewHub returns an empty Hub.
func NewHub(m *metrics.Metrics) *Hub {
	return &Hub{scopes: map[string]map[*Subscription]struct{}{}, metrics: m}
}

// Subscription receives the versions a scope reaches. Versions the reader
// has not taken yet are coalesced into the highest one.
type Subscription struct {
	hub   *Hub
	scope string
	ch    chan uint64
}

// Pokes yields versions; it is closed when the Hub closes.
func (s *Subscription) Pokes() <-chan uint64 {
	return s.ch
}

// Close stops the subscription.
func (s *Subscription) Close() {
	s.hub.remove(s)
}

// Subscribe starts a subscription to scope. It reports false once the Hub
// has closed.
func (h *Hub) Subscribe(scope string) (*Subscription, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, false
	}
	s := &Subscription{hub: h, scope: scope, ch: make(chan uint64, 1)}
	subs, ok := h.scopes[scope]
	if !ok {
		subs = map[*Subscription]struct{}{}
		h.scopes[scope] = subs
	}
	subs[s] = struct{}{}
	return s, true
}

// Deliver hands n to every subscription of its scope without blocking.
func (h *Hub) Deliver(n Notification) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.scopes[n.Scope] {
		s.offer(n.Version)
		h.metrics.PokeFanOut.Inc()
	}
}

// Subscribers returns how many subscriptions scope has.
func (h *Hub) Subscribers(scope string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.scopes[scope])
}

// Close ends every subscription and refuses new ones.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for _, subs := range h.scopes {
		for s := range subs {
			close(s.ch)
		}
	}
	clear(h.scopes)
}

func (h *Hub) remove(s *Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs := h.scopes[s.scope]
	if _, ok := subs[s]; !ok {
		return
	}
	delete(subs, s)
	if len(subs) == 0 {
		delete(h.scopes, s.scope)
	}
	close(s.ch)
}

func (s *Subscription) offer(version uint64) {
	for {
		select {
		case s.ch <- version:
			return
		default:
		}
		select {
		case pending := <-s.ch:
			version = max(version, pending)
		default:
		}
	}
}
