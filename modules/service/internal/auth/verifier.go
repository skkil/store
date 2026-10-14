package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Audience is the only audience the store service accepts.
const Audience = "skkil-store"

// MaxLifetime is the longest a sync token may live, from iat to exp.
const MaxLifetime = 15 * time.Minute

const clockSkew = 30 * time.Second

// ErrInvalidToken wraps every reason a token is refused.
var ErrInvalidToken = errors.New("invalid sync token")

// Verifier checks ES256 sync tokens from one issuer.
type Verifier struct {
	parser *jwt.Parser
	keys   jwt.Keyfunc
}

type syncClaims struct {
	jwt.RegisteredClaims
	ClientID string `json:"cid"`
}

// NewVerifier returns a Verifier for tokens issued by issuer and signed by a
// key that keys resolves. now supplies the clock and may be nil for time.Now.
func NewVerifier(issuer string, keys jwt.Keyfunc, now func() time.Time) *Verifier {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(Audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(clockSkew),
	}
	if now != nil {
		opts = append(opts, jwt.WithTimeFunc(now))
	}
	return &Verifier{parser: jwt.NewParser(opts...), keys: keys}
}

// Verify checks a raw token and returns the identity it grants.
func (v *Verifier) Verify(raw string) (Identity, error) {
	var claims syncClaims
	if _, err := v.parser.ParseWithClaims(raw, &claims, v.keys); err != nil {
		return Identity{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if claims.IssuedAt == nil || claims.ExpiresAt.Sub(claims.IssuedAt.Time) > MaxLifetime {
		return Identity{}, fmt.Errorf("%w: lifetime exceeds %s", ErrInvalidToken, MaxLifetime)
	}
	scope, err := uuid.Parse(claims.Subject)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: sub is not a scope id", ErrInvalidToken)
	}
	client, err := uuid.Parse(claims.ClientID)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: cid is not a client id", ErrInvalidToken)
	}
	return Identity{Scope: scope.String(), Client: client.String()}, nil
}

// VerifyHeader checks an "Authorization: Bearer <token>" header value.
func (v *Verifier) VerifyHeader(header string) (Identity, error) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return Identity{}, fmt.Errorf("%w: expected a bearer token", ErrInvalidToken)
	}
	return v.Verify(strings.TrimSpace(token))
}
