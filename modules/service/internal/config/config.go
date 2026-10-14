// Package config reads the service configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Config is everything the service needs to start.
type Config struct {
	DatabaseURL       string
	ListenAddr        string
	TokenIssuer       string
	TokenPublicKeyPEM string
	TokenJWKSURL      string
	AllowedOrigins    []string
	PollInterval      time.Duration
	PokeKeepAlive     time.Duration
	MinVersionWait    time.Duration
	ShutdownTimeout   time.Duration
	LogLevel          slog.Level
}

const (
	defaultListenAddr      = ":8080"
	defaultPollInterval    = 5 * time.Second
	defaultPokeKeepAlive   = 25 * time.Second
	defaultMinVersionWait  = 2 * time.Second
	defaultShutdownTimeout = 20 * time.Second
)

// Load builds a Config from environment variables read through getenv,
// usually os.Getenv.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DatabaseURL:       strings.TrimSpace(getenv("STORE_DATABASE_URL")),
		ListenAddr:        valueOr(getenv("STORE_LISTEN_ADDR"), defaultListenAddr),
		TokenIssuer:       strings.TrimSpace(getenv("STORE_TOKEN_ISSUER")),
		TokenPublicKeyPEM: strings.TrimSpace(getenv("STORE_TOKEN_PUBLIC_KEY_PEM")),
		TokenJWKSURL:      strings.TrimSpace(getenv("STORE_TOKEN_JWKS_URL")),
		AllowedOrigins:    splitList(getenv("STORE_ALLOWED_ORIGINS")),
	}
	var errs []error
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("STORE_DATABASE_URL is required"))
	}
	if cfg.TokenIssuer == "" {
		errs = append(errs, errors.New("STORE_TOKEN_ISSUER is required"))
	}
	switch {
	case cfg.TokenPublicKeyPEM == "" && cfg.TokenJWKSURL == "":
		errs = append(errs, errors.New("one of STORE_TOKEN_PUBLIC_KEY_PEM or STORE_TOKEN_JWKS_URL is required"))
	case cfg.TokenPublicKeyPEM != "" && cfg.TokenJWKSURL != "":
		errs = append(errs, errors.New("set only one of STORE_TOKEN_PUBLIC_KEY_PEM and STORE_TOKEN_JWKS_URL"))
	}
	var err error
	if cfg.PollInterval, err = duration(getenv, "STORE_POLL_INTERVAL", defaultPollInterval); err != nil {
		errs = append(errs, err)
	}
	if cfg.PokeKeepAlive, err = duration(getenv, "STORE_POKE_KEEPALIVE", defaultPokeKeepAlive); err != nil {
		errs = append(errs, err)
	}
	if cfg.MinVersionWait, err = duration(getenv, "STORE_MIN_VERSION_WAIT", defaultMinVersionWait); err != nil {
		errs = append(errs, err)
	}
	if cfg.ShutdownTimeout, err = duration(getenv, "STORE_SHUTDOWN_TIMEOUT", defaultShutdownTimeout); err != nil {
		errs = append(errs, err)
	}
	if level := strings.TrimSpace(getenv("STORE_LOG_LEVEL")); level != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(level)); err != nil {
			errs = append(errs, fmt.Errorf("STORE_LOG_LEVEL: %w", err))
		}
	}
	return cfg, errors.Join(errs...)
}

func duration(getenv func(string) string, name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(getenv(name))
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}
	return d, nil
}

func valueOr(raw, fallback string) string {
	if v := strings.TrimSpace(raw); v != "" {
		return v
	}
	return fallback
}

func splitList(raw string) []string {
	var out []string
	for part := range strings.SplitSeq(raw, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}
