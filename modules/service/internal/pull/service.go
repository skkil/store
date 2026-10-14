package pull

import (
	"context"
	"errors"
	"time"

	storev1 "github.com/skkil/store/modules/service/gen/storev1"
	"github.com/skkil/store/modules/service/internal/auth"
)

// Limits on how many versions one pull returns.
const (
	DefaultLimit = 500
	MaxLimit     = 1000
)

// ErrBehind means the log has not reached the request's min_version within
// the wait; the client should retry.
var ErrBehind = errors.New("log is behind min_version")

// Request is one pull for the caller's scope.
type Request struct {
	Cursor     Cursor
	MinVersion uint64
	Limit      uint32
}

// Service answers pulls from the log.
type Service struct {
	log     *Log
	maxWait time.Duration
}

// NewService returns a Service that waits up to maxWait for min_version.
func NewService(log *Log, maxWait time.Duration) *Service {
	return &Service{log: log, maxWait: maxWait}
}

// Pull returns the versions after req.Cursor for id's scope, or a Reset when
// the cursor no longer matches the log.
func (s *Service) Pull(ctx context.Context, id auth.Identity, req Request) (*storev1.PullResponse, error) {
	deadline := time.Now().Add(s.maxWait)
	backoff := 10 * time.Millisecond
	for {
		resp, err := s.read(ctx, id, req)
		if !errors.Is(err, ErrBehind) || time.Now().Add(backoff).After(deadline) {
			return resp, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 200*time.Millisecond)
	}
}

func (s *Service) read(ctx context.Context, id auth.Identity, req Request) (*storev1.PullResponse, error) {
	var resp *storev1.PullResponse
	err := s.log.View(ctx, func(v *View) error {
		head, err := v.Head(ctx, id.Scope)
		if err != nil {
			return err
		}
		last, err := v.LastMutationID(ctx, id.Scope, id.Client)
		if err != nil {
			return err
		}
		var logHash []byte
		if req.Cursor.Version > 0 && req.Cursor.Version <= head.Version {
			if logHash, err = v.HashAt(ctx, id.Scope, req.Cursor.Version); err != nil {
				return err
			}
		}
		resp = &storev1.PullResponse{LastMutationId: last, HeadVersion: head.Version, Epoch: head.Epoch}
		if reason := resetReason(head, req.Cursor, logHash); reason != "" {
			resp.Body = &storev1.PullResponse_Reset_{Reset_: &storev1.Reset{Epoch: head.Epoch, Reason: reason}}
			return nil
		}
		if head.Version < req.MinVersion {
			return ErrBehind
		}
		through := min(head.Version, req.Cursor.Version+uint64(limit(req.Limit)))
		versions, err := v.Versions(ctx, id.Scope, req.Cursor.Version, through)
		if err != nil {
			return err
		}
		resp.Body = &storev1.PullResponse_Entries{Entries: &storev1.Entries{Versions: versions, More: through < head.Version}}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func limit(requested uint32) uint32 {
	switch {
	case requested == 0:
		return DefaultLimit
	case requested > MaxLimit:
		return MaxLimit
	default:
		return requested
	}
}
