package server

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"
)

// loggingInterceptor logs every RPC: failures at warn, the rest at debug.
type loggingInterceptor struct {
	logger *slog.Logger
}

func (l loggingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()
		resp, err := next(ctx, req)
		l.log(ctx, req.Spec().Procedure, req.Peer().Protocol, start, err)
		return resp, err
	}
}

func (l loggingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (l loggingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		start := time.Now()
		err := next(ctx, conn)
		l.log(ctx, conn.Spec().Procedure, conn.Peer().Protocol, start, err)
		return err
	}
}

func (l loggingInterceptor) log(ctx context.Context, procedure, protocol string, start time.Time, err error) {
	attrs := []any{"procedure", procedure, "protocol", protocol, "duration", time.Since(start)}
	if err != nil {
		code := connect.CodeOf(err)
		attrs = append(attrs, "code", code.String(), "error", err)
		if code == connect.CodeInternal || code == connect.CodeUnknown {
			l.logger.ErrorContext(ctx, "rpc", attrs...)
			return
		}
		l.logger.WarnContext(ctx, "rpc", attrs...)
		return
	}
	l.logger.DebugContext(ctx, "rpc", attrs...)
}
