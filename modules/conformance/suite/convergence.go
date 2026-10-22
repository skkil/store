package suite

import (
	"bytes"
	"context"
	"time"

	storev1 "github.com/skkil/store/modules/conformance/gen/storev1"
	"github.com/skkil/store/modules/conformance/suite/check"
	"github.com/skkil/store/modules/conformance/suite/sim"
)

const (
	convergePause = 20 * time.Millisecond
	pokePoll      = 20 * time.Millisecond
)

func (e *execution) converge(ctx context.Context) error {
	auditor, err := e.reachHead(ctx)
	if err != nil {
		return err
	}
	head := auditor.Cursor()
	e.awaitPokes(head.GetVersion())
	states := make([]check.ClientState, len(e.clients))
	for i, c := range e.clients {
		states[i] = check.ClientState{Name: c.Name(), Pending: c.PendingCount(), Cursor: c.Cursor(), State: c.State()}
	}
	e.ledger.Audit(check.Converged{Log: auditor.Log(), Head: head, State: auditor.State(), Clients: states})
	return nil
}

func (e *execution) reachHead(ctx context.Context) (*sim.Client, error) {
	var auditor *sim.Client
	for round := 1; round <= e.cfg.ConvergeRounds; round++ {
		for _, c := range e.clients {
			if !c.Online() {
				summary, err := c.Connect(ctx)
				if err != nil {
					return nil, err
				}
				e.log("converge %d %s online: %s", round, c.Name(), summary)
			}
			summary, err := c.Push(ctx, false)
			if err != nil {
				return nil, err
			}
			e.log("converge %d %s push: %s", round, c.Name(), summary)
		}
		for _, c := range e.clients {
			summary, err := c.PullAll(ctx)
			if err != nil {
				return nil, err
			}
			e.log("converge %d %s pull: %s", round, c.Name(), summary)
		}
		auditor = e.newClient("auditor", false, true)
		if err := auditor.Start(ctx); err != nil {
			return nil, err
		}
		summary, err := auditor.PullAll(ctx)
		if err != nil {
			return nil, err
		}
		e.log("converge %d auditor pull: %s", round, summary)
		if e.settled(auditor.Cursor()) {
			return auditor, nil
		}
		time.Sleep(convergePause)
	}
	return auditor, nil
}

func (e *execution) settled(head *storev1.Cursor) bool {
	for _, c := range e.clients {
		if c.PendingCount() > 0 || c.Cursor().GetVersion() != head.GetVersion() || !bytes.Equal(c.Cursor().GetHash(), head.GetHash()) {
			return false
		}
	}
	return true
}

func (e *execution) awaitPokes(head uint64) {
	deadline := time.Now().Add(e.cfg.PokeTimeout)
	for _, c := range e.clients {
		if !c.Listening() || head == 0 {
			continue
		}
		for c.HighestPoke() < head && time.Now().Before(deadline) {
			time.Sleep(pokePoll)
		}
		if c.HighestPoke() < head {
			e.ledger.Violate(check.PokesReachHead, c.Name(), "highest poke v%d after %s, head is v%d", c.HighestPoke(), e.cfg.PokeTimeout, head)
		} else {
			e.log("%s poked up to v%d", c.Name(), c.HighestPoke())
		}
	}
}
