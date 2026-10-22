package stacktest

import (
	"testing"

	"github.com/skkil/store/modules/conformance/reference"
)

// IssuerName is the iss of the reference product's tokens in tests.
const IssuerName = "reference"

// Stack is one database, one store service and the token issuer they share.
type Stack struct {
	DB     *Database
	Issuer *reference.TokenIssuer
	Store  *StoreService
}

// NewStack starts a database and a store service trusting a fresh issuer.
func NewStack(t testing.TB) *Stack {
	t.Helper()
	key, err := reference.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	issuer := reference.NewTokenIssuer(IssuerName, key)
	publicKey, err := issuer.PublicKeyPEM()
	if err != nil {
		t.Fatal(err)
	}
	db := NewDatabase(t)
	return &Stack{DB: db, Issuer: issuer, Store: StartStoreService(t, db.URL, IssuerName, publicKey)}
}

// Product starts a reference product with fault on the stack's database.
func (s *Stack) Product(t testing.TB, fault reference.Fault) *Product {
	t.Helper()
	return StartProduct(t, s.DB, s.Issuer, fault)
}
