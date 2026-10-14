// Command store runs the store read service: Pull and Pokes over Connect.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/skkil/store/modules/service/internal/app"
	"github.com/skkil/store/modules/service/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("store service failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service, err := app.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer service.Close()
	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return err
	}
	return service.Serve(ctx, ln)
}
