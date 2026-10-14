// Package server exposes StoreService, health and metrics over HTTP.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"

	storev1 "github.com/skkil/store/modules/service/gen/storev1"
	"github.com/skkil/store/modules/service/gen/storev1/storev1connect"
	"github.com/skkil/store/modules/service/internal/auth"
	"github.com/skkil/store/modules/service/internal/metrics"
	"github.com/skkil/store/modules/service/internal/poke"
	"github.com/skkil/store/modules/service/internal/pull"
)

// ProtocolVersion is the only protocol_version this service speaks.
const ProtocolVersion = 1

// StoreHandler implements skkil.store.v1.StoreService.
type StoreHandler struct {
	verifier  *auth.Verifier
	pulls     *pull.Service
	log       *pull.Log
	hub       *poke.Hub
	metrics   *metrics.Metrics
	logger    *slog.Logger
	keepAlive time.Duration
}

var _ storev1connect.StoreServiceHandler = (*StoreHandler)(nil)

// StoreHandlerConfig lists what a StoreHandler depends on.
type StoreHandlerConfig struct {
	Verifier  *auth.Verifier
	Pulls     *pull.Service
	Log       *pull.Log
	Hub       *poke.Hub
	Metrics   *metrics.Metrics
	Logger    *slog.Logger
	KeepAlive time.Duration
}

// NewStoreHandler returns a StoreHandler.
func NewStoreHandler(cfg StoreHandlerConfig) *StoreHandler {
	return &StoreHandler{
		verifier:  cfg.Verifier,
		pulls:     cfg.Pulls,
		log:       cfg.Log,
		hub:       cfg.Hub,
		metrics:   cfg.Metrics,
		logger:    cfg.Logger,
		keepAlive: cfg.KeepAlive,
	}
}

// Pull implements StoreServiceHandler.
func (h *StoreHandler) Pull(ctx context.Context, req *connect.Request[storev1.PullRequest]) (*connect.Response[storev1.PullResponse], error) {
	start := time.Now()
	resp, err := h.pull(ctx, req)
	outcome := "entries"
	switch {
	case err != nil:
		outcome = "error"
	case resp.GetReset_() != nil:
		outcome = "reset"
		h.metrics.Resets.WithLabelValues(resp.GetReset_().GetReason()).Inc()
	default:
		h.metrics.VersionsServed.Add(float64(len(resp.GetEntries().GetVersions())))
	}
	h.metrics.PullDuration.WithLabelValues(outcome).Observe(time.Since(start).Seconds())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *StoreHandler) pull(ctx context.Context, req *connect.Request[storev1.PullRequest]) (*storev1.PullResponse, error) {
	msg := req.Msg
	id, err := h.authorize(req.Header(), msg.GetProtocolVersion(), msg.GetClientId())
	if err != nil {
		return nil, err
	}
	cursor := msg.GetCursor()
	resp, err := h.pulls.Pull(ctx, id, pull.Request{
		Cursor:     pull.Cursor{Epoch: cursor.GetEpoch(), Version: cursor.GetVersion(), Hash: cursor.GetHash()},
		MinVersion: msg.GetMinVersion(),
		Limit:      msg.GetLimit(),
	})
	if err != nil {
		return nil, h.internal(ctx, "pull", err)
	}
	return resp, nil
}

// Pokes implements StoreServiceHandler.
func (h *StoreHandler) Pokes(ctx context.Context, req *connect.Request[storev1.PokesRequest], stream *connect.ServerStream[storev1.Poke]) error {
	id, err := h.authorize(req.Header(), req.Msg.GetProtocolVersion(), req.Msg.GetClientId())
	if err != nil {
		return err
	}
	sub, ok := h.hub.Subscribe(id.Scope)
	if !ok {
		return connect.NewError(connect.CodeUnavailable, errors.New("shutting down"))
	}
	defer sub.Close()
	h.metrics.ActiveStreams.Inc()
	defer h.metrics.ActiveStreams.Dec()

	head, err := h.log.Head(ctx, id.Scope)
	if err != nil {
		return h.internal(ctx, "pokes", err)
	}
	last := head.Version
	if err := h.send(stream, last); err != nil {
		return nil
	}
	ticker := time.NewTicker(h.keepAlive)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case version, open := <-sub.Pokes():
			if !open {
				return nil
			}
			if version <= last {
				continue
			}
			last = version
			ticker.Reset(h.keepAlive)
		case <-ticker.C:
		}
		if err := h.send(stream, last); err != nil {
			return nil
		}
	}
}

func (h *StoreHandler) send(stream *connect.ServerStream[storev1.Poke], version uint64) error {
	if err := stream.Send(&storev1.Poke{Version: version}); err != nil {
		return err
	}
	h.metrics.PokesSent.Inc()
	return nil
}

func (h *StoreHandler) authorize(header http.Header, protocolVersion uint32, clientID string) (auth.Identity, error) {
	id, err := h.verifier.VerifyHeader(header.Get("Authorization"))
	if err != nil {
		return auth.Identity{}, connect.NewError(connect.CodeUnauthenticated, err)
	}
	if protocolVersion != ProtocolVersion {
		return auth.Identity{}, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("protocol_version %d is not supported; this server speaks %d", protocolVersion, ProtocolVersion))
	}
	if clientID != id.Client {
		return auth.Identity{}, connect.NewError(connect.CodePermissionDenied, errors.New("client_id does not match the sync token"))
	}
	return id, nil
}

func (h *StoreHandler) internal(ctx context.Context, op string, err error) error {
	switch {
	case errors.Is(err, pull.ErrBehind):
		return connect.NewError(connect.CodeUnavailable, err)
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, err)
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, err)
	}
	h.logger.ErrorContext(ctx, op+" failed", "error", err)
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}
