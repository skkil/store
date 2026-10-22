// Package scenario builds the seeded scripts the conformance suite runs:
// sequences of client steps for offline windows, concurrent writes,
// duplicate pushes, rejections, poke loss, small pulls and reconnects.
package scenario

import (
	"fmt"
	"strings"

	"github.com/skkil/store/modules/conformance/suite/profile"
)

// Op is what one step does.
type Op int

// The steps a scenario is made of.
const (
	// Mutate makes one mutation on the client and queues it.
	Mutate Op = iota
	// Push sends the client's pending queue.
	Push
	// PushLost sends the pending queue and drops the response.
	PushLost
	// Pull fetches one page of at most Limit versions.
	Pull
	// PullAll pulls until the client has the head, as on a timer.
	PullAll
	// Offline takes the client offline.
	Offline
	// Online reconnects the client: Pokes stream and a pull.
	Online
	// React pulls if a poke announced a newer version.
	React
	// Concurrent runs Group at the same time, one step per client.
	Concurrent
	// StaleHash replaces the cursor's hash; the next pull must Reset.
	StaleHash
	// StaleAhead moves the cursor past the head; the next pull must Reset.
	StaleAhead
	// BumpEpoch bumps the scope's epoch, as a restore or failover does;
	// every non-fresh cursor must Reset.
	BumpEpoch
	// Settle waits briefly so pokes in flight arrive.
	Settle
)

var opNames = map[Op]string{
	Mutate: "mutate", Push: "push", PushLost: "push-lost", Pull: "pull", PullAll: "pull-all",
	Offline: "offline", Online: "online", React: "react", Concurrent: "concurrent",
	StaleHash: "stale-hash", StaleAhead: "stale-ahead", BumpEpoch: "bump-epoch", Settle: "settle",
}

func (o Op) String() string { return opNames[o] }

// Step is one action of a scenario.
type Step struct {
	Op     Op
	Client int
	// Limit is the page size of a Pull.
	Limit uint32
	// Seed picks and fills the mutation of a Mutate, so removing other steps
	// never changes what this one makes.
	Seed uint64
	// Kind restricts a Mutate to one kind of mutation; "" allows any.
	Kind  profile.Kind
	Group []Step
}

func (s Step) String() string {
	switch s.Op {
	case Concurrent:
		parts := make([]string, len(s.Group))
		for i, g := range s.Group {
			parts[i] = g.String()
		}
		return "concurrent{" + strings.Join(parts, " | ") + "}"
	case BumpEpoch, Settle:
		return s.Op.String()
	case Pull:
		return fmt.Sprintf("c%d pull limit=%d", s.Client, s.Limit)
	case Mutate:
		if s.Kind != "" {
			return fmt.Sprintf("c%d mutate %s", s.Client, s.Kind)
		}
		return fmt.Sprintf("c%d mutate", s.Client)
	default:
		return fmt.Sprintf("c%d %s", s.Client, s.Op)
	}
}
