// Package poke tells open streams that a scope reached a new version.
package poke

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Notification says a scope reached a version.
type Notification struct {
	Scope   string
	Version uint64
}

// ParseNotification reads a store_poke payload, "<scope uuid>:<version>".
func ParseNotification(payload string) (Notification, error) {
	scope, version, ok := strings.Cut(payload, ":")
	if !ok {
		return Notification{}, fmt.Errorf("poke payload %q: missing version", payload)
	}
	id, err := uuid.Parse(scope)
	if err != nil {
		return Notification{}, fmt.Errorf("poke payload %q: %w", payload, err)
	}
	v, err := strconv.ParseUint(version, 10, 64)
	if err != nil {
		return Notification{}, fmt.Errorf("poke payload %q: %w", payload, err)
	}
	return Notification{Scope: id.String(), Version: v}, nil
}
