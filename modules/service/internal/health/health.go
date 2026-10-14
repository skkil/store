// Package health serves the liveness and readiness probes.
package health

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

const pingTimeout = 2 * time.Second

// Checker answers /livez and /readyz.
type Checker struct {
	ping     func(context.Context) error
	draining atomic.Bool
}

// New returns a Checker that is ready while ping succeeds.
func New(ping func(context.Context) error) *Checker {
	return &Checker{ping: ping}
}

// Drain makes readiness fail from now on, so load balancers stop routing
// new requests here during shutdown.
func (c *Checker) Drain() {
	c.draining.Store(true)
}

// Live answers 200 while the process serves HTTP.
func (c *Checker) Live(w http.ResponseWriter, _ *http.Request) {
	write(w, http.StatusOK, "ok")
}

// Ready answers 200 when the database answers and the service is not draining.
func (c *Checker) Ready(w http.ResponseWriter, r *http.Request) {
	if c.draining.Load() {
		write(w, http.StatusServiceUnavailable, "draining")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
	defer cancel()
	if err := c.ping(ctx); err != nil {
		write(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	write(w, http.StatusOK, "ok")
}

func write(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body + "\n"))
}
