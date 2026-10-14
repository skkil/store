package server

import (
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"
	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"

	"github.com/skkil/store/modules/service/gen/storev1/storev1connect"
	"github.com/skkil/store/modules/service/internal/health"
	"github.com/skkil/store/modules/service/internal/metrics"
)

const maxRequestBytes = 1 << 20

// Routes lists what the HTTP handler serves.
type Routes struct {
	Store          *StoreHandler
	Health         *health.Checker
	Metrics        *metrics.Metrics
	Logger         *slog.Logger
	AllowedOrigins []string
}

// NewHandler mounts StoreService (Connect, gRPC and gRPC-Web), /livez,
// /readyz and /metrics, with CORS for the allowed browser origins.
func NewHandler(r Routes) http.Handler {
	mux := http.NewServeMux()
	path, store := storev1connect.NewStoreServiceHandler(r.Store,
		connect.WithInterceptors(loggingInterceptor{logger: r.Logger}),
		connect.WithReadMaxBytes(maxRequestBytes),
	)
	mux.Handle(path, withCORS(store, r.AllowedOrigins))
	mux.HandleFunc("GET /livez", r.Health.Live)
	mux.HandleFunc("GET /readyz", r.Health.Ready)
	mux.Handle("GET /metrics", r.Metrics.Handler())
	return mux
}

func withCORS(h http.Handler, origins []string) http.Handler {
	if len(origins) == 0 {
		return h
	}
	return cors.New(cors.Options{
		AllowedOrigins: origins,
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: append(connectcors.AllowedHeaders(), "Authorization"),
		ExposedHeaders: connectcors.ExposedHeaders(),
		MaxAge:         int((2 * time.Hour).Seconds()),
	}).Handler(h)
}

// NewHTTPServer returns a server for handler that speaks HTTP/1.1 and
// cleartext HTTP/2 (h2c) on the same port. Write timeouts stay off because
// Pokes streams are long-lived.
func NewHTTPServer(handler http.Handler, logger *slog.Logger) *http.Server {
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	return &http.Server{
		Handler:           handler,
		Protocols:         &protocols,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
}
