// Command reference runs the reference product server: a todo list whose
// Push goes through the Go write path, plus its sync-token endpoint.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skkil/store/modules/conformance/reference"
)

const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, logger, os.Args[1:]); err != nil {
		logger.Error("reference failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, args []string) error {
	flags := flag.NewFlagSet("reference", flag.ContinueOnError)
	databaseURL := flags.String("database-url", os.Getenv("REFERENCE_DATABASE_URL"), "Postgres with the store migrations applied")
	listen := flags.String("listen", ":8081", "listen address")
	issuer := flags.String("issuer", "reference", "sync-token issuer (iss)")
	keyPath := flags.String("signing-key", "", "P-256 private key PEM; default: a fresh key")
	publicKeyOut := flags.String("public-key-out", "", "write the token verification key (PEM) to this file")
	migrate := flags.Bool("migrate", false, "apply the reference schema first")
	faultName := flags.String("fault", string(reference.FaultNone), "deliberate fault, for testing the suite")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *databaseURL == "" {
		return errors.New("-database-url or REFERENCE_DATABASE_URL is required")
	}
	fault, err := reference.ParseFault(*faultName)
	if err != nil {
		return err
	}
	tokens, err := tokenIssuer(*issuer, *keyPath)
	if err != nil {
		return err
	}
	if err := writePublicKey(tokens, *publicKeyOut); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, *databaseURL)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	if *migrate {
		if err := reference.Migrate(ctx, pool); err != nil {
			return err
		}
	}
	handler, err := reference.NewHandler(ctx, reference.Config{Pool: pool, Issuer: tokens, Logger: logger, Fault: fault})
	if err != nil {
		return err
	}
	return serve(ctx, logger, *listen, handler)
}

func tokenIssuer(name, keyPath string) (*reference.TokenIssuer, error) {
	if keyPath == "" {
		key, err := reference.GenerateKey()
		if err != nil {
			return nil, err
		}
		return reference.NewTokenIssuer(name, key), nil
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	key, err := reference.ParsePrivateKeyPEM(data)
	if err != nil {
		return nil, err
	}
	return reference.NewTokenIssuer(name, key), nil
}

func writePublicKey(tokens *reference.TokenIssuer, path string) error {
	if path == "" {
		return nil
	}
	pem, err := tokens.PublicKeyPEM()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(pem), 0o644)
}

func serve(ctx context.Context, logger *slog.Logger, addr string, handler http.Handler) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	srv := &http.Server{Handler: handler, Protocols: &protocols, ReadHeaderTimeout: 10 * time.Second}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	logger.Info("reference listening", "addr", ln.Addr().String())
	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
