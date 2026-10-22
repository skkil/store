package stacktest

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/skkil/store/modules/conformance/reference"
)

// Product is a reference product server running in-process.
type Product struct {
	URL      string
	TokenURL string
}

// StartProduct serves the reference product with fault on db, signing
// tokens with issuer.
func StartProduct(t testing.TB, db *Database, issuer *reference.TokenIssuer, fault reference.Fault) *Product {
	t.Helper()
	handler, err := reference.NewHandler(context.Background(), reference.Config{
		Pool:   db.Pool,
		Issuer: issuer,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Fault:  fault,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Product{URL: server.URL, TokenURL: server.URL + reference.TokenPath}
}
