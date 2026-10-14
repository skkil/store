package auth_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/skkil/store/modules/service/internal/auth"
	"github.com/skkil/store/modules/service/internal/storetest"
)

func staticVerifier(t *testing.T, issuer *storetest.Issuer) *auth.Verifier {
	t.Helper()
	keys, err := auth.StaticKey(issuer.PublicKeyPEM(t))
	if err != nil {
		t.Fatal(err)
	}
	return auth.NewVerifier(issuer.Name, keys, nil)
}

func claims(scope, client string, mutate func(jwt.MapClaims)) jwt.MapClaims {
	now := time.Now()
	c := jwt.MapClaims{
		"iss": "time",
		"aud": "skkil-store",
		"sub": scope,
		"cid": client,
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	}
	if mutate != nil {
		mutate(c)
	}
	return c
}

func TestVerifyAcceptsAValidToken(t *testing.T) {
	issuer := storetest.NewIssuer(t, "time")
	scope, client := uuid.NewString(), uuid.NewString()
	id, err := staticVerifier(t, issuer).VerifyHeader("Bearer " + issuer.Token(t, scope, client))
	if err != nil {
		t.Fatal(err)
	}
	if id != (auth.Identity{Scope: scope, Client: client}) {
		t.Fatalf("identity %+v", id)
	}
}

func TestVerifyNormalisesIDs(t *testing.T) {
	issuer := storetest.NewIssuer(t, "time")
	scope, client := uuid.New(), uuid.New()
	id, err := staticVerifier(t, issuer).Verify(issuer.Token(t, strings.ToUpper(scope.String()), strings.ToUpper(client.String())))
	if err != nil {
		t.Fatal(err)
	}
	if id.Scope != scope.String() || id.Client != client.String() {
		t.Fatalf("identity %+v", id)
	}
}

func TestVerifyRefusesBadTokens(t *testing.T) {
	issuer := storetest.NewIssuer(t, "time")
	other := storetest.NewIssuer(t, "time")
	verifier := staticVerifier(t, issuer)
	scope, client := uuid.NewString(), uuid.NewString()
	hs256, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims(scope, client, nil)).SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"wrong issuer":   issuer.Sign(t, claims(scope, client, func(c jwt.MapClaims) { c["iss"] = "clip" })),
		"wrong audience": issuer.Sign(t, claims(scope, client, func(c jwt.MapClaims) { c["aud"] = "time-api" })),
		"expired": issuer.Sign(t, claims(scope, client, func(c jwt.MapClaims) {
			c["iat"] = time.Now().Add(-10 * time.Minute).Unix()
			c["exp"] = time.Now().Add(-5 * time.Minute).Unix()
		})),
		"no expiry":      issuer.Sign(t, claims(scope, client, func(c jwt.MapClaims) { delete(c, "exp") })),
		"no issued at":   issuer.Sign(t, claims(scope, client, func(c jwt.MapClaims) { delete(c, "iat") })),
		"issued later":   issuer.Sign(t, claims(scope, client, func(c jwt.MapClaims) { c["iat"] = time.Now().Add(5 * time.Minute).Unix() })),
		"too long-lived": issuer.Sign(t, claims(scope, client, func(c jwt.MapClaims) { c["exp"] = time.Now().Add(16 * time.Minute).Unix() })),
		"bad signature":  other.Sign(t, claims(scope, client, nil)),
		"tampered":       tamper(issuer.Sign(t, claims(scope, client, nil))),
		"hs256":          hs256,
		"unsigned":       unsigned(t, claims(scope, client, nil)),
		"bad scope":      issuer.Sign(t, claims("me", client, nil)),
		"missing client": issuer.Sign(t, claims(scope, client, func(c jwt.MapClaims) { delete(c, "cid") })),
		"garbage":        "not-a-token",
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := verifier.Verify(token)
			if !errors.Is(err, auth.ErrInvalidToken) {
				t.Fatalf("got %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestVerifyHeaderRequiresBearer(t *testing.T) {
	issuer := storetest.NewIssuer(t, "time")
	verifier := staticVerifier(t, issuer)
	token := issuer.Token(t, uuid.NewString(), uuid.NewString())
	for _, header := range []string{"", token, "Basic " + token, "Bearer ", "Bearer"} {
		if _, err := verifier.VerifyHeader(header); !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("header %q: got %v", header, err)
		}
	}
	if _, err := verifier.VerifyHeader("bearer " + token); err != nil {
		t.Errorf("lower-case scheme: %v", err)
	}
}

func TestVerifyHonoursTheClock(t *testing.T) {
	issuer := storetest.NewIssuer(t, "time")
	keys, err := auth.StaticKey(issuer.PublicKeyPEM(t))
	if err != nil {
		t.Fatal(err)
	}
	token := issuer.Token(t, uuid.NewString(), uuid.NewString())
	later := auth.NewVerifier("time", keys, func() time.Time { return time.Now().Add(time.Hour) })
	if _, err := later.Verify(token); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("got %v", err)
	}
}

func TestStaticKeyRejectsOtherKeys(t *testing.T) {
	for name, text := range map[string]string{
		"empty":   "",
		"garbage": "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----\n",
		"p384":    p384PEM(t),
	} {
		if _, err := auth.StaticKey(text); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestJWKSResolvesByKeyID(t *testing.T) {
	issuer := storetest.NewIssuer(t, "time")
	other := storetest.NewIssuer(t, "time")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(issuer.JWKS(t))
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	keys, err := auth.JWKS(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	verifier := auth.NewVerifier("time", keys, nil)
	scope, client := uuid.NewString(), uuid.NewString()
	if _, err := verifier.Verify(issuer.Token(t, scope, client)); err != nil {
		t.Fatalf("valid token: %v", err)
	}
	other.KeyID = issuer.KeyID
	if _, err := verifier.Verify(other.Token(t, scope, client)); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("foreign key: %v", err)
	}
}

func tamper(token string) string {
	parts := strings.Split(token, ".")
	parts[1] = parts[1][:len(parts[1])-2] + "AA"
	return strings.Join(parts, ".")
}

func unsigned(t *testing.T, c jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, c).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func p384PEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}
