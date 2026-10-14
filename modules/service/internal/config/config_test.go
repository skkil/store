package config

import (
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
)

func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"STORE_DATABASE_URL":         "postgres://localhost/db",
		"STORE_TOKEN_ISSUER":         "time",
		"STORE_TOKEN_PUBLIC_KEY_PEM": "pem",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":8080" || cfg.PollInterval != 5*time.Second || cfg.ShutdownTimeout != 20*time.Second {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.PokeKeepAlive != 25*time.Second || cfg.MinVersionWait != 2*time.Second {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.LogLevel != slog.LevelInfo || cfg.AllowedOrigins != nil {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadReadsEveryVariable(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"STORE_DATABASE_URL":     "postgres://localhost/db",
		"STORE_LISTEN_ADDR":      "127.0.0.1:9000",
		"STORE_TOKEN_ISSUER":     "time",
		"STORE_TOKEN_JWKS_URL":   "https://time.example/jwks.json",
		"STORE_ALLOWED_ORIGINS":  " https://a.example , ,https://b.example",
		"STORE_POLL_INTERVAL":    "750ms",
		"STORE_SHUTDOWN_TIMEOUT": "3s",
		"STORE_POKE_KEEPALIVE":   "10s",
		"STORE_MIN_VERSION_WAIT": "500ms",
		"STORE_LOG_LEVEL":        "debug",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != "127.0.0.1:9000" || cfg.TokenJWKSURL != "https://time.example/jwks.json" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.PokeKeepAlive != 10*time.Second || cfg.MinVersionWait != 500*time.Millisecond {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if !slices.Equal(cfg.AllowedOrigins, []string{"https://a.example", "https://b.example"}) {
		t.Fatalf("origins: %v", cfg.AllowedOrigins)
	}
	if cfg.PollInterval != 750*time.Millisecond || cfg.ShutdownTimeout != 3*time.Second || cfg.LogLevel != slog.LevelDebug {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadRejectsMissingAndConflictingValues(t *testing.T) {
	cases := map[string]struct {
		values map[string]string
		want   []string
	}{
		"empty": {
			values: map[string]string{},
			want:   []string{"STORE_DATABASE_URL", "STORE_TOKEN_ISSUER", "one of STORE_TOKEN_PUBLIC_KEY_PEM"},
		},
		"both keys": {
			values: map[string]string{
				"STORE_DATABASE_URL":         "postgres://localhost/db",
				"STORE_TOKEN_ISSUER":         "time",
				"STORE_TOKEN_PUBLIC_KEY_PEM": "pem",
				"STORE_TOKEN_JWKS_URL":       "https://time.example/jwks.json",
			},
			want: []string{"set only one"},
		},
		"bad durations and level": {
			values: map[string]string{
				"STORE_DATABASE_URL":         "postgres://localhost/db",
				"STORE_TOKEN_ISSUER":         "time",
				"STORE_TOKEN_PUBLIC_KEY_PEM": "pem",
				"STORE_POLL_INTERVAL":        "soon",
				"STORE_SHUTDOWN_TIMEOUT":     "-1s",
				"STORE_LOG_LEVEL":            "loud",
			},
			want: []string{"STORE_POLL_INTERVAL", "STORE_SHUTDOWN_TIMEOUT must be positive", "STORE_LOG_LEVEL"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(env(tc.values))
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}
