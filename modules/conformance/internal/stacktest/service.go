package stacktest

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

var (
	binaryOnce sync.Once
	binaryPath string
	binaryErr  error
)

// StoreService is a store service subprocess built from modules/service.
type StoreService struct {
	URL string
}

// StartStoreService builds cmd/store once per test binary and runs it
// against databaseURL, trusting tokens from issuer signed by publicKeyPEM.
func StartStoreService(t testing.TB, databaseURL, issuer, publicKeyPEM string) *StoreService {
	t.Helper()
	binaryOnce.Do(func() { binaryPath, binaryErr = buildStoreService() })
	if binaryErr != nil {
		t.Fatalf("build store service: %v", binaryErr)
	}
	addr := freeAddr(t)
	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(),
		"STORE_DATABASE_URL="+databaseURL,
		"STORE_LISTEN_ADDR="+addr,
		"STORE_TOKEN_ISSUER="+issuer,
		"STORE_TOKEN_PUBLIC_KEY_PEM="+publicKeyPEM,
		"STORE_POLL_INTERVAL=500ms",
		"STORE_MIN_VERSION_WAIT=500ms",
		"STORE_SHUTDOWN_TIMEOUT=2s",
		"STORE_LOG_LEVEL=warn",
	)
	var logs strings.Builder
	cmd.Stdout, cmd.Stderr = &logs, &logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-exited
		}
		if t.Failed() {
			t.Logf("store service logs:\n%s", logs.String())
		}
	})
	s := &StoreService{URL: "http://" + addr}
	if err := s.awaitReady(exited); err != nil {
		t.Fatalf("store service: %v\n%s", err, logs.String())
	}
	return s
}

func buildStoreService() (string, error) {
	dir, err := os.MkdirTemp("", "conformance-store-*")
	if err != nil {
		return "", err
	}
	out := filepath.Join(dir, "store")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/store")
	cmd.Dir = filepath.Join(ModuleRoot(), "..", "service")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w: %s", err, output)
	}
	return out, nil
}

func (s *StoreService) awaitReady(exited <-chan error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		select {
		case err := <-exited:
			return fmt.Errorf("exited: %v", err)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
		resp, err := http.Get(s.URL + "/readyz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

func freeAddr(t testing.TB) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}
