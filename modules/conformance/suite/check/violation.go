package check

import "fmt"

// Violation is one broken invariant.
type Violation struct {
	Invariant Invariant
	// Client is the simulated client that observed it, or "" for a check
	// over the whole scope.
	Client  string
	Message string
}

func (v Violation) String() string {
	if v.Client == "" {
		return fmt.Sprintf("%s: %s", v.Invariant, v.Message)
	}
	return fmt.Sprintf("%s [%s]: %s", v.Invariant, v.Client, v.Message)
}
