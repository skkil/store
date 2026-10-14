package storetest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Issuer signs sync tokens the way a product server does.
type Issuer struct {
	Name  string
	KeyID string
	key   *ecdsa.PrivateKey
}

// NewIssuer returns an Issuer named name with a fresh P-256 key.
func NewIssuer(t testing.TB, name string) *Issuer {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &Issuer{Name: name, KeyID: name + "-1", key: key}
}

// Token returns a valid five-minute token for scope and client.
func (i *Issuer) Token(t testing.TB, scope, client string) string {
	t.Helper()
	now := time.Now()
	return i.Sign(t, jwt.MapClaims{
		"iss": i.Name,
		"aud": "skkil-store",
		"sub": scope,
		"cid": client,
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	})
}

// Sign signs arbitrary claims with ES256.
func (i *Issuer) Sign(t testing.TB, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = i.KeyID
	signed, err := token.SignedString(i.key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

// PublicKeyPEM returns the verification key as PKIX PEM.
func (i *Issuer) PublicKeyPEM(t testing.TB) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(&i.key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// JWKS returns the verification key as a JSON Web Key Set.
func (i *Issuer) JWKS(t testing.TB) []byte {
	t.Helper()
	ecdh, err := i.key.PublicKey.ECDH()
	if err != nil {
		t.Fatal(err)
	}
	point := ecdh.Bytes()
	encode := base64.RawURLEncoding.EncodeToString
	set := map[string]any{"keys": []map[string]string{{
		"kty": "EC",
		"crv": "P-256",
		"alg": "ES256",
		"use": "sig",
		"kid": i.KeyID,
		"x":   encode(point[1:33]),
		"y":   encode(point[33:]),
	}}}
	raw, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
