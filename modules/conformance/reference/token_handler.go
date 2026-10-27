package reference

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skkil/store/modules/conformance/writepath"
)

// TokenRequest asks for a sync token for one client of one user.
type TokenRequest struct {
	User     string `json:"user"`
	ClientID string `json:"client_id"`
}

// TokenResponse carries a sync token and the scope it was issued for.
type TokenResponse struct {
	Token   string `json:"token"`
	ScopeID string `json:"scope_id"`
}

// scopeNamespace turns a user name into a stable scope id.
var scopeNamespace = uuid.MustParse("6f3c2a4e-2c1b-4f53-9d1e-5d4b2f0c8a71")

// ScopeOf returns the scope of user: one personal scope per user, as in v1.
func ScopeOf(user string) uuid.UUID {
	return uuid.NewSHA1(scopeNamespace, []byte(user))
}

// TokenHandler serves POST /sync-token. The reference product has no
// sign-in, so it trusts the user named in the request; a real product issues
// tokens only to the signed-in user. A client id already bound to another
// user's scope is refused with 409.
type TokenHandler struct {
	issuer *TokenIssuer
	pool   *pgxpool.Pool
}

// NewTokenHandler returns a TokenHandler signing with issuer and claiming
// clients in pool.
func NewTokenHandler(issuer *TokenIssuer, pool *pgxpool.Pool) *TokenHandler {
	return &TokenHandler{issuer: issuer, pool: pool}
}

func (h *TokenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, "expected {\"user\", \"client_id\"}", http.StatusBadRequest)
		return
	}
	client, err := uuid.Parse(req.ClientID)
	if err != nil || req.User == "" {
		http.Error(w, "user must be set and client_id must be a UUID", http.StatusBadRequest)
		return
	}
	scope := ScopeOf(req.User)
	claimed, err := writepath.ClaimClient(r.Context(), h.pool, scope, client)
	if err != nil {
		http.Error(w, "could not claim the client", http.StatusInternalServerError)
		return
	}
	if !claimed {
		http.Error(w, "client_id belongs to another user", http.StatusConflict)
		return
	}
	token, err := h.issuer.Issue(scope, client)
	if err != nil {
		http.Error(w, "could not sign the token", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(TokenResponse{Token: token, ScopeID: scope.String()})
}
