package scenario

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"slices"
)

// Scenario is one script for a fresh scope.
type Scenario struct {
	Name    string
	Clients int
	// Listening says which clients hold a Pokes stream and act on pokes.
	Listening []bool
	Steps     []Step
}

// Options size the generated scenarios.
type Options struct {
	// Clients is how many simulated clients share the scope (at least 2).
	Clients int
	// Steps is roughly how many steps a scenario has.
	Steps int
	// EpochBumps allows BumpEpoch steps; it needs database access.
	EpochBumps bool
}

type generator func(b *builder)

var generators = map[string]generator{
	"offline":     offline,
	"concurrent":  concurrent,
	"duplicates":  duplicates,
	"rejections":  rejections,
	"poke-loss":   pokeLoss,
	"small-pulls": smallPulls,
	"reconnect":   reconnect,
	"random":      random,
}

// Names lists every scenario in a fixed order.
func Names() []string {
	return []string{"offline", "concurrent", "duplicates", "rejections", "poke-loss", "small-pulls", "reconnect", "random"}
}

// Build generates the scenario name for seed. The same name, seed and
// options always give the same steps.
func Build(name string, seed uint64, opts Options) (Scenario, error) {
	gen, ok := generators[name]
	if !ok {
		return Scenario{}, fmt.Errorf("unknown scenario %q (have %v)", name, Names())
	}
	opts.Clients = max(opts.Clients, 2)
	opts.Steps = max(opts.Steps, 10)
	h := fnv.New64a()
	h.Write([]byte(name))
	b := &builder{
		rng:  rand.New(rand.NewPCG(seed, h.Sum64())),
		opts: opts,
		s:    Scenario{Name: name, Clients: opts.Clients, Listening: slices.Repeat([]bool{true}, opts.Clients)},
	}
	gen(b)
	return b.s, nil
}

// WithSteps returns a copy of s running steps instead of its own.
func (s Scenario) WithSteps(steps []Step) Scenario {
	s.Steps = steps
	return s
}
