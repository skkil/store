package reference

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenAudience is the aud of every sync token.
const TokenAudience = "skkil-store"

// TokenTTL is how long an issued token lives; the protocol allows at most 15 minutes.
const TokenTTL = 15 * time.Minute

// Identity is who a verified sync token speaks for.
type Identity struct {
	Scope  uuid.UUID
	Client uuid.UUID
}

// TokenIssuer signs and verifies ES256 sync tokens (docs/protocol.md, Sync token).
type TokenIssuer struct {
	name string
	key  *ecdsa.PrivateKey
	now  func() time.Time
}

// NewTokenIssuer returns an issuer named name signing with key.
func NewTokenIssuer(name string, key *ecdsa.PrivateKey) *TokenIssuer {
	return &TokenIssuer{name: name, key: key, now: time.Now}
}

// GenerateKey returns a fresh P-256 signing key.
func GenerateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

// ParsePrivateKeyPEM reads a PKCS#8 or SEC 1 PEM P-256 private key.
func ParsePrivateKeyPEM(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("no PEM block")
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("private key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || key.Curve != elliptic.P256() {
		return nil, errors.New("private key: not a P-256 key")
	}
	return key, nil
}

// Name is the iss of every token.
func (i *TokenIssuer) Name() string { return i.name }

// Issue returns a token that lets client pull and receive pokes for scope.
func (i *TokenIssuer) Issue(scope, client uuid.UUID) (string, error) {
	now := i.now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": i.name,
		"aud": TokenAudience,
		"sub": scope.String(),
		"cid": client.String(),
		"iat": now.Unix(),
		"exp": now.Add(TokenTTL).Unix(),
	})
	return token.SignedString(i.key)
}

// Verify checks a token this issuer signed and returns its identity.
func (i *TokenIssuer) Verify(raw string) (Identity, error) {
	parsed, err := jwt.Parse(raw,
		func(*jwt.Token) (any, error) { return &i.key.PublicKey, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}),
		jwt.WithIssuer(i.name),
		jwt.WithAudience(TokenAudience),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(i.now),
	)
	if err != nil {
		return Identity{}, err
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Identity{}, errors.New("token: unexpected claims")
	}
	sub, _ := claims["sub"].(string)
	cid, _ := claims["cid"].(string)
	scope, err := uuid.Parse(sub)
	if err != nil {
		return Identity{}, fmt.Errorf("token: sub: %w", err)
	}
	client, err := uuid.Parse(cid)
	if err != nil {
		return Identity{}, fmt.Errorf("token: cid: %w", err)
	}
	return Identity{Scope: scope, Client: client}, nil
}

// PublicKeyPEM returns the verification key as PKIX PEM for STORE_TOKEN_PUBLIC_KEY_PEM.
func (i *TokenIssuer) PublicKeyPEM() (string, error) {
	der, err := x509.MarshalPKIXPublicKey(&i.key.PublicKey)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}
