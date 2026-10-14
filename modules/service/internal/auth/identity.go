// Package auth verifies the sync tokens product servers issue to their clients.
package auth

// Identity is what a verified sync token grants: reading one scope as one client.
type Identity struct {
	Scope  string
	Client string
}
