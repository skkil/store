package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skkil/store/modules/conformance/internal/stacktest"
	"github.com/skkil/store/modules/conformance/reference"
)

func profilePath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, reference.Profile, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunPassesAgainstTheReference(t *testing.T) {
	stack := stacktest.NewStack(t)
	product := stack.Product(t, reference.FaultNone)
	var out strings.Builder
	code, err := run(context.Background(), []string{
		"-push-url", product.URL,
		"-store-url", stack.Store.URL,
		"-token-url", product.TokenURL,
		"-profile", profilePath(t),
		"-database-url", stack.DB.URL,
		"-seed", "5",
		"-scenarios", "reconnect,random",
	}, &out)
	if err != nil || code != 0 {
		t.Fatalf("exit %d, %v:\n%s", code, err, out.String())
	}
	if !strings.Contains(out.String(), "seed 5") || !strings.Contains(out.String(), "ok: 2 runs passed") {
		t.Fatalf("output:\n%s", out.String())
	}
}

func TestRunTakesTokensFromACommand(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl is not installed")
	}
	stack := stacktest.NewStack(t)
	product := stack.Product(t, reference.FaultNone)
	command := `curl -sf -X POST -d "{\"user\":\"$STORE_CONFORMANCE_USER\",\"client_id\":\"$STORE_CONFORMANCE_CLIENT_ID\"}" ` + product.TokenURL
	var out strings.Builder
	code, err := run(context.Background(), []string{
		"-push-url", product.URL, "-store-url", stack.Store.URL, "-token-cmd", command,
		"-profile", profilePath(t), "-scenarios", "duplicates", "-steps", "20",
	}, &out)
	if err != nil || code != 0 {
		t.Fatalf("exit %d, %v:\n%s", code, err, out.String())
	}
}

func TestRunExitsOneWhenTheProductIsBroken(t *testing.T) {
	stack := stacktest.NewStack(t)
	product := stack.Product(t, reference.FaultIgnoreDuplicates)
	var out strings.Builder
	code, err := run(context.Background(), []string{
		"-push-url", product.URL, "-store-url", stack.Store.URL, "-token-url", product.TokenURL,
		"-profile", profilePath(t), "-scenarios", "duplicates", "-seed", "3", "-shrink-runs", "5",
	}, &out)
	if err != nil || code != exitFailed {
		t.Fatalf("exit %d, %v:\n%s", code, err, out.String())
	}
	if !strings.Contains(out.String(), "reproduce: -seed 3 -runs 1 -scenarios duplicates") {
		t.Fatalf("output:\n%s", out.String())
	}
}

func TestRunRefusesBadUsage(t *testing.T) {
	profile := profilePath(t)
	for name, args := range map[string][]string{
		"no urls":       {"-profile", profile, "-token-url", "http://x"},
		"no tokens":     {"-push-url", "http://x", "-store-url", "http://y", "-profile", profile},
		"both tokens":   {"-push-url", "http://x", "-store-url", "http://y", "-profile", profile, "-token-url", "http://t", "-token-cmd", "true"},
		"bad header":    {"-push-url", "http://x", "-store-url", "http://y", "-profile", profile, "-token-url", "http://t", "-token-header", "nocolon"},
		"bad profile":   {"-push-url", "http://x", "-store-url", "http://y", "-profile", profile + ".missing", "-token-cmd", "true"},
		"unknown flag":  {"-frobnicate"},
		"bad scenarios": {"-push-url", "http://x", "-store-url", "http://y", "-profile", profile, "-token-cmd", "true", "-scenarios", "chaos"},
	} {
		code, _ := run(context.Background(), args, &strings.Builder{})
		if code != exitUsage {
			t.Errorf("%s: exit %d, want %d", name, code, exitUsage)
		}
	}
}
