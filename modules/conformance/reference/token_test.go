package reference_test

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/skkil/store/modules/conformance/reference"
)

func TestTokenEndpointIssuesVerifiableTokensPerUser(t *testing.T) {
	h := newHarness(t)
	client := uuid.New()
	resp, err := http.Post(h.url+reference.TokenPath, "application/json",
		strings.NewReader(`{"user":"ada","client_id":"`+client.String()+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %s", resp.Status)
	}
	var body reference.TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	identity, err := h.issuer.Verify(body.Token)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Scope != reference.ScopeOf("ada") || identity.Client != client || body.ScopeID != identity.Scope.String() {
		t.Fatalf("identity = %+v, body = %+v", identity, body)
	}
	if reference.ScopeOf("ada") == reference.ScopeOf("bob") {
		t.Fatal("two users share a scope")
	}
}

func TestTokenEndpointRefusesAClientOfAnotherUser(t *testing.T) {
	h := newHarness(t)
	client := uuid.NewString()
	for _, tc := range []struct {
		user string
		want int
	}{{"ada", http.StatusOK}, {"ada", http.StatusOK}, {"bob", http.StatusConflict}} {
		resp, err := http.Post(h.url+reference.TokenPath, "application/json",
			strings.NewReader(`{"user":"`+tc.user+`","client_id":"`+client+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Fatalf("%s: status %s, want %d", tc.user, resp.Status, tc.want)
		}
	}
}

func TestTokenEndpointRejectsBadRequests(t *testing.T) {
	h := newHarness(t)
	for _, body := range []string{`nope`, `{"user":"","client_id":"` + uuid.NewString() + `"}`, `{"user":"ada","client_id":"x"}`} {
		resp, err := http.Post(h.url+reference.TokenPath, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status %s, want 400", body, resp.Status)
		}
	}
}

func TestPublicKeyEndpointServesThePEM(t *testing.T) {
	h := newHarness(t)
	resp, err := http.Get(h.url + reference.PublicKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatalf("not PEM: %s", data)
	}
	if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
		t.Fatal(err)
	}
}

func TestPrivateKeyPEMRoundTrips(t *testing.T) {
	key, err := reference.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	for _, encode := range []func() ([]byte, string){
		func() ([]byte, string) { der, _ := x509.MarshalPKCS8PrivateKey(key); return der, "PRIVATE KEY" },
		func() ([]byte, string) { der, _ := x509.MarshalECPrivateKey(key); return der, "EC PRIVATE KEY" },
	} {
		der, kind := encode()
		parsed, err := reference.ParsePrivateKeyPEM(pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}))
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if !parsed.Equal(key) {
			t.Fatalf("%s: a different key came back", kind)
		}
	}
	if _, err := reference.ParsePrivateKeyPEM([]byte("garbage")); err == nil {
		t.Fatal("garbage parsed as a key")
	}
}

func TestParseFault(t *testing.T) {
	for _, f := range reference.Faults() {
		got, err := reference.ParseFault(string(f))
		if err != nil || got != f {
			t.Fatalf("ParseFault(%q) = %q, %v", f, got, err)
		}
	}
	if _, err := reference.ParseFault("gremlins"); err == nil {
		t.Fatal("unknown fault accepted")
	}
}
