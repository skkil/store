package scenario

import (
	"math/rand/v2"

	"github.com/skkil/store/modules/conformance/suite/profile"
)

type builder struct {
	rng  *rand.Rand
	opts Options
	s    Scenario
}

func (b *builder) add(steps ...Step) {
	b.s.Steps = append(b.s.Steps, steps...)
}

func (b *builder) full() bool {
	return len(b.s.Steps) >= b.opts.Steps
}

func (b *builder) client() int {
	return b.rng.IntN(b.s.Clients)
}

func (b *builder) mutate(client int) Step {
	return Step{Op: Mutate, Client: client, Seed: b.rng.Uint64()}
}

func (b *builder) mutateKind(client int, kind profile.Kind) Step {
	return Step{Op: Mutate, Client: client, Seed: b.rng.Uint64(), Kind: kind}
}

func (b *builder) mutations(client, n int) {
	for range n {
		b.add(b.mutate(client))
	}
}

func (b *builder) concurrently(op Op, clients []int) {
	group := make([]Step, len(clients))
	for i, c := range clients {
		group[i] = Step{Op: op, Client: c}
	}
	b.add(Step{Op: Concurrent, Group: group})
}

func (b *builder) everyone() []int {
	all := make([]int, b.s.Clients)
	for i := range all {
		all[i] = i
	}
	return all
}

func (b *builder) smallLimit() uint32 {
	return uint32(1 + b.rng.IntN(3))
}
