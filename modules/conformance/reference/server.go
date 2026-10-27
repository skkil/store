package reference

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skkil/store/modules/conformance/gen/storev1/storev1connect"
)

// TokenPath is where the reference product issues sync tokens.
const TokenPath = "/sync-token"

// PublicKeyPath serves the sync-token verification key as PEM.
const PublicKeyPath = "/sync-token/public-key"

// Config configures a reference product server.
type Config struct {
	Pool   *pgxpool.Pool
	Issuer *TokenIssuer
	Logger *slog.Logger
	Fault  Fault
}

// NewHandler returns the reference product's HTTP handler: PushService over
// Connect, the token endpoint and the public key.
func NewHandler(ctx context.Context, cfg Config) (http.Handler, error) {
	fault := cfg.Fault
	if fault == "" {
		fault = FaultNone
	}
	applier, err := NewApplier(ctx, cfg.Pool, cfg.Logger, fault)
	if err != nil {
		return nil, err
	}
	publicKey, err := cfg.Issuer.PublicKeyPEM()
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle(storev1connect.NewPushServiceHandler(NewPushHandler(cfg.Issuer, Todos(), applier, cfg.Logger)))
	mux.Handle("POST "+TokenPath, NewTokenHandler(cfg.Issuer, cfg.Pool))
	mux.HandleFunc("GET "+PublicKeyPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		_, _ = w.Write([]byte(publicKey))
	})
	return mux, nil
}
