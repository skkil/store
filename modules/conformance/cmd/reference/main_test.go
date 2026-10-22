package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/skkil/store/modules/conformance/internal/stacktest"
	"github.com/skkil/store/modules/conformance/reference"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestRunServesUntilCancelled(t *testing.T) {
	db := stacktest.NewDatabase(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	publicKey := filepath.Join(t.TempDir(), "public.pem")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, discard, []string{"-database-url", db.URL, "-listen", addr, "-public-key-out", publicKey, "-fault", "none"})
	}()
	body := `{"user":"ada","client_id":"` + uuid.NewString() + `"}`
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := http.Post("http://"+addr+reference.TokenPath, "application/json", strings.NewReader(body))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("token endpoint answered %s", resp.Status)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never answered: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pem, err := os.ReadFile(publicKey); err != nil || !strings.Contains(string(pem), "PUBLIC KEY") {
		t.Fatalf("public key file: %v %q", err, pem)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRunRefusesBadFlags(t *testing.T) {
	ctx := context.Background()
	if err := run(ctx, discard, []string{"-database-url", ""}); err == nil {
		t.Error("ran without a database")
	}
	if err := run(ctx, discard, []string{"-database-url", "postgres://x", "-fault", "gremlins"}); err == nil {
		t.Error("ran with an unknown fault")
	}
	if err := run(ctx, discard, []string{"-database-url", "postgres://x", "-signing-key", "/nonexistent"}); err == nil {
		t.Error("ran with a missing key file")
	}
}
