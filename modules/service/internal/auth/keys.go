package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// StaticKey resolves every token to one P-256 public key given in PEM
// (PKIX "PUBLIC KEY").
func StaticKey(pemText string) (jwt.Keyfunc, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, errors.New("token public key: no PEM block")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("token public key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok || key.Curve != elliptic.P256() {
		return nil, errors.New("token public key: expected an ECDSA P-256 key")
	}
	return func(*jwt.Token) (any, error) { return key, nil }, nil
}

// JWKS resolves tokens by key id against a JSON Web Key Set at url, which is
// fetched now and refreshed in the background until ctx ends.
func JWKS(ctx context.Context, url string) (jwt.Keyfunc, error) {
	set, err := keyfunc.NewDefaultCtx(ctx, []string{url})
	if err != nil {
		return nil, fmt.Errorf("token JWKS: %w", err)
	}
	return set.Keyfunc, nil
}
