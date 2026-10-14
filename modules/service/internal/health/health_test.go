package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func status(handler http.HandlerFunc) int {
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec.Code
}

func TestReadyFollowsThePing(t *testing.T) {
	var pingErr error
	c := New(func(context.Context) error { return pingErr })
	if got := status(c.Ready); got != http.StatusOK {
		t.Fatalf("ready: %d", got)
	}
	pingErr = errors.New("down")
	if got := status(c.Ready); got != http.StatusServiceUnavailable {
		t.Fatalf("db down: %d", got)
	}
	if got := status(c.Live); got != http.StatusOK {
		t.Fatalf("live while db down: %d", got)
	}
}

func TestDrainFailsReadiness(t *testing.T) {
	c := New(func(context.Context) error { return nil })
	c.Drain()
	if got := status(c.Ready); got != http.StatusServiceUnavailable {
		t.Fatalf("draining: %d", got)
	}
	if got := status(c.Live); got != http.StatusOK {
		t.Fatalf("live while draining: %d", got)
	}
}
