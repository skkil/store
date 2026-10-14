package e2e

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	storev1 "github.com/skkil/store/modules/service/gen/storev1"
	"github.com/skkil/store/modules/service/internal/storetest"
)

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body)
}

func TestHealthAndMetrics(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	inst := start(t, baseConfig(t, db, issuer))
	d := newDevice(t, issuer)
	if _, err := inst.client().Pull(context.Background(), pullRequest(d, nil)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/livez", "/readyz"} {
		if code, _ := get(t, inst.url+path); code != http.StatusOK {
			t.Fatalf("%s: %d", path, code)
		}
	}
	code, body := get(t, inst.url+"/metrics")
	if code != http.StatusOK {
		t.Fatalf("metrics: %d", code)
	}
	for _, name := range []string{
		`store_pull_duration_seconds_count{outcome="entries"} 1`,
		"store_pull_versions_served_total 0",
		"store_poke_streams_active 0",
		"store_poke_fanout_total",
		"go_goroutines",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("metrics lack %q", name)
		}
	}
}

func TestCORSAllowsConfiguredOrigins(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	cfg := baseConfig(t, db, issuer)
	cfg.AllowedOrigins = []string{"https://time.example"}
	inst := start(t, cfg)
	preflight := func(origin string) *http.Response {
		req, err := http.NewRequest(http.MethodOptions, inst.url+"/skkil.store.v1.StoreService/Pull", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "authorization,connect-protocol-version,content-type")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp
	}
	allowed := preflight("https://time.example")
	if allowed.Header.Get("Access-Control-Allow-Origin") != "https://time.example" {
		t.Fatalf("allowed origin: %v", allowed.Header)
	}
	headers := strings.ToLower(allowed.Header.Get("Access-Control-Allow-Headers"))
	for _, h := range []string{"authorization", "connect-protocol-version", "content-type"} {
		if !strings.Contains(headers, h) {
			t.Fatalf("allowed headers %q lack %s", headers, h)
		}
	}
	if denied := preflight("https://evil.example"); denied.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("foreign origin allowed: %v", denied.Header)
	}
}

func TestJWKSConfiguration(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(issuer.JWKS(t))
	}))
	t.Cleanup(jwks.Close)
	cfg := baseConfig(t, db, issuer)
	cfg.TokenPublicKeyPEM = ""
	cfg.TokenJWKSURL = jwks.URL
	inst := start(t, cfg)
	d := newDevice(t, issuer)
	if _, err := inst.client().Pull(context.Background(), pullRequest(d, nil)); err != nil {
		t.Fatal(err)
	}
}

func cursorOf(v *storev1.Version, epoch uint32) *storev1.Cursor {
	return &storev1.Cursor{Epoch: epoch, Version: v.GetVersion(), Hash: v.GetHash()}
}
