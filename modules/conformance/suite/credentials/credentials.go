// Package credentials gets the sync tokens (and Push headers) the
// conformance suite's simulated clients need, from a URL or a command.
package credentials

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Credentials let one client of one user push and pull.
type Credentials struct {
	// Token is the sync token for Pull and Pokes.
	Token string
	// Scope is the token's sub claim.
	Scope string
	// PushHeaders go on every Push request. Without push_headers in the
	// provider's answer they are "Authorization: Bearer <Token>".
	PushHeaders http.Header
	// Expiry is the token's exp claim.
	Expiry time.Time
}

// Source fetches credentials for client of user. Every client of one user
// must get a token for the same scope.
type Source interface {
	Fetch(ctx context.Context, user, client string) (Credentials, error)
}

type answer struct {
	Token       string            `json:"token"`
	PushHeaders map[string]string `json:"push_headers"`
}

type claims struct {
	Sub string `json:"sub"`
	Cid string `json:"cid"`
	Exp int64  `json:"exp"`
}

// Parse reads a provider's answer: either a bare JWT or a JSON object
// {"token": "<jwt>", "push_headers": {"Name": "value"}}.
func Parse(output []byte, client string) (Credentials, error) {
	output = bytes.TrimSpace(output)
	var a answer
	if bytes.HasPrefix(output, []byte("{")) {
		if err := json.Unmarshal(output, &a); err != nil {
			return Credentials{}, fmt.Errorf("credentials: %w", err)
		}
	} else {
		a.Token = string(output)
	}
	if a.Token == "" {
		return Credentials{}, errors.New("credentials: empty token")
	}
	c, err := readClaims(a.Token)
	if err != nil {
		return Credentials{}, err
	}
	if c.Cid != client {
		return Credentials{}, fmt.Errorf("credentials: token cid %q, want %q", c.Cid, client)
	}
	if c.Sub == "" {
		return Credentials{}, errors.New("credentials: token has no sub (scope)")
	}
	headers := http.Header{}
	if len(a.PushHeaders) == 0 {
		headers.Set("Authorization", "Bearer "+a.Token)
	}
	for name, value := range a.PushHeaders {
		headers.Set(name, value)
	}
	return Credentials{Token: a.Token, Scope: c.Sub, PushHeaders: headers, Expiry: time.Unix(c.Exp, 0)}, nil
}

func readClaims(token string) (claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims{}, errors.New("credentials: token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims{}, fmt.Errorf("credentials: token payload: %w", err)
	}
	var c claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return claims{}, fmt.Errorf("credentials: token payload: %w", err)
	}
	return c, nil
}
