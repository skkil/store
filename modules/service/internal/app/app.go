// Package app assembles the service from its configuration and runs it.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skkil/store/modules/service/internal/auth"
	"github.com/skkil/store/modules/service/internal/config"
	"github.com/skkil/store/modules/service/internal/health"
	"github.com/skkil/store/modules/service/internal/metrics"
	"github.com/skkil/store/modules/service/internal/poke"
	"github.com/skkil/store/modules/service/internal/pull"
	"github.com/skkil/store/modules/service/internal/server"
)

const startupPingTimeout = 10 * time.Second

// App is one running store service instance.
type App struct {
	cfg     config.Config
	logger  *slog.Logger
	pool    *pgxpool.Pool
	hub     *poke.Hub
	listen  *poke.ListenBus
	poll    *poke.PollBus
	health  *health.Checker
	metrics *metrics.Metrics
	handler http.Handler
	cancel  context.CancelFunc
}

// New connects to the database, loads the token keys and wires every
// component. Close releases what New acquired.
func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	life, cancel := context.WithCancel(context.WithoutCancel(ctx))
	a := &App{cfg: cfg, logger: logger, cancel: cancel, metrics: metrics.New()}
	if err := a.init(ctx, life); err != nil {
		a.Close()
		return nil, err
	}
	return a, nil
}

func (a *App) init(ctx, life context.Context) error {
	keys, err := tokenKeys(life, a.cfg)
	if err != nil {
		return err
	}
	if a.pool, err = pgxpool.New(life, a.cfg.DatabaseURL); err != nil {
		return fmt.Errorf("database: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, startupPingTimeout)
	defer cancel()
	if err := a.pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("database: %w", err)
	}
	log := pull.NewLog(a.pool)
	a.hub = poke.NewHub(a.metrics)
	a.listen = poke.NewListenBus(a.cfg.DatabaseURL, a.logger)
	a.poll = poke.NewPollBus(a.pool, a.cfg.PollInterval, poke.DefaultLookback, a.logger)
	a.health = health.New(a.pool.Ping)
	a.handler = server.NewHandler(server.Routes{
		Store: server.NewStoreHandler(server.StoreHandlerConfig{
			Verifier:  auth.NewVerifier(a.cfg.TokenIssuer, keys, nil),
			Pulls:     pull.NewService(log, a.cfg.MinVersionWait),
			Log:       log,
			Hub:       a.hub,
			Metrics:   a.metrics,
			Logger:    a.logger,
			KeepAlive: a.cfg.PokeKeepAlive,
		}),
		Health:         a.health,
		Metrics:        a.metrics,
		Logger:         a.logger,
		AllowedOrigins: a.cfg.AllowedOrigins,
	})
	return nil
}

func tokenKeys(ctx context.Context, cfg config.Config) (jwt.Keyfunc, error) {
	if cfg.TokenJWKSURL != "" {
		return auth.JWKS(ctx, cfg.TokenJWKSURL)
	}
	return auth.StaticKey(cfg.TokenPublicKeyPEM)
}

// Handler is the instance's HTTP handler.
func (a *App) Handler() http.Handler {
	return a.handler
}

// Serve runs the poke buses and serves HTTP on ln until ctx ends, then shuts
// down gracefully: readiness fails, streams end, in-flight pulls finish.
func (a *App) Serve(ctx context.Context, ln net.Listener) error {
	busCtx, stopBuses := context.WithCancel(context.WithoutCancel(ctx))
	var buses sync.WaitGroup
	for source, bus := range map[string]poke.PokeBus{"listen": a.listen, "poll": a.poll} {
		notifications := a.metrics.BusNotifications.WithLabelValues(source)
		buses.Go(func() {
			_ = bus.Run(busCtx, func(n poke.Notification) {
				notifications.Inc()
				a.hub.Deliver(n)
			})
		})
	}
	defer func() {
		stopBuses()
		buses.Wait()
	}()

	srv := server.NewHTTPServer(a.handler, a.logger)
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	a.logger.Info("store service listening", "addr", ln.Addr().String())

	select {
	case err := <-served:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}
	a.logger.Info("store service shutting down")
	a.health.Drain()
	a.hub.Close()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	a.logger.Info("store service stopped")
	return nil
}

// Close releases the database pool and background key refresh.
func (a *App) Close() {
	a.cancel()
	if a.pool != nil {
		a.pool.Close()
	}
}
