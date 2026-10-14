package poke

import "context"

// PokeBus carries scope notifications from every writer to this instance.
// Delivery is at most once; the clients' pulls recover anything lost.
type PokeBus interface {
	// Run calls deliver for each notification until ctx ends.
	Run(ctx context.Context, deliver func(Notification)) error
}
