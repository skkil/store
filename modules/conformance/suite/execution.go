package suite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/skkil/store/modules/conformance/suite/check"
	"github.com/skkil/store/modules/conformance/suite/scenario"
	"github.com/skkil/store/modules/conformance/suite/sim"
)

const settleDelay = 50 * time.Millisecond

type execution struct {
	cfg       Config
	sc        scenario.Scenario
	seed      uint64
	transport *sim.Transport
	ledger    *check.Ledger
	user      string
	scope     string
	clients   []*sim.Client
	trace     []string
}

func execute(ctx context.Context, cfg Config, transport *sim.Transport, sc scenario.Scenario, seed uint64) Result {
	start := time.Now()
	e := &execution{
		cfg:       cfg,
		sc:        sc,
		seed:      seed,
		transport: transport,
		ledger:    check.NewLedger(),
		user:      fmt.Sprintf("conformance-%s-%d-%s", sc.Name, seed, nonce()),
	}
	defer e.close()
	ctx, cancel := context.WithTimeout(ctx, cfg.RunTimeout)
	defer cancel()
	err := e.run(ctx)
	return Result{
		Scenario:   sc.Name,
		Seed:       seed,
		Steps:      len(sc.Steps),
		Duration:   time.Since(start),
		Violations: e.ledger.Violations(),
		Trace:      e.trace,
		Err:        err,
	}
}

func (e *execution) run(ctx context.Context) error {
	if err := e.startClients(ctx); err != nil {
		return err
	}
	for i, step := range e.sc.Steps {
		if err := e.step(ctx, i+1, step); err != nil {
			return err
		}
	}
	return e.converge(ctx)
}

func (e *execution) newClient(name string, listen, keepLog bool) *sim.Client {
	return sim.New(sim.Options{
		Name:        name,
		User:        e.user,
		Profile:     e.cfg.Profile,
		Transport:   e.transport,
		Credentials: e.cfg.Credentials,
		Ledger:      e.ledger,
		ListenPokes: listen,
		PageSize:    e.cfg.PageSize,
		KeepLog:     keepLog,
	})
}

func (e *execution) startClients(ctx context.Context) error {
	for i := range e.sc.Clients {
		c := e.newClient(fmt.Sprintf("c%d", i), e.sc.Listening[i], false)
		e.clients = append(e.clients, c)
		creds, err := e.cfg.Credentials.Fetch(ctx, e.user, c.ID())
		if err != nil {
			return err
		}
		if e.scope == "" {
			e.scope = creds.Scope
		} else if creds.Scope != e.scope {
			return fmt.Errorf("credentials: clients of user %s got scopes %s and %s", e.user, e.scope, creds.Scope)
		}
		if err := c.Start(ctx); err != nil {
			return err
		}
	}
	e.log("scope %s, user %s, %d clients", e.scope, e.user, len(e.clients))
	return nil
}

func (e *execution) step(ctx context.Context, n int, step scenario.Step) error {
	if step.Op == scenario.Concurrent {
		return e.concurrent(ctx, n, step)
	}
	summary, err := e.do(ctx, step)
	if err != nil {
		return fmt.Errorf("step %d (%s): %w", n, step, err)
	}
	e.log("#%d %s: %s", n, step, summary)
	return nil
}

func (e *execution) concurrent(ctx context.Context, n int, step scenario.Step) error {
	summaries := make([]string, len(step.Group))
	errs := make([]error, len(step.Group))
	var wg sync.WaitGroup
	for i, g := range step.Group {
		wg.Go(func() { summaries[i], errs[i] = e.do(ctx, g) })
	}
	wg.Wait()
	for i, g := range step.Group {
		if errs[i] != nil {
			return fmt.Errorf("step %d (%s): %w", n, g, errs[i])
		}
		e.log("#%d %s: %s", n, g, summaries[i])
	}
	return nil
}

func (e *execution) do(ctx context.Context, step scenario.Step) (string, error) {
	if step.Op == scenario.BumpEpoch {
		return e.bumpEpoch(ctx)
	}
	if step.Op == scenario.Settle {
		time.Sleep(settleDelay)
		return "waited", nil
	}
	c := e.clients[step.Client%len(e.clients)]
	switch step.Op {
	case scenario.Mutate:
		return c.Mutate(step.Seed, step.Kind), nil
	case scenario.Push:
		return c.Push(ctx, false)
	case scenario.PushLost:
		return c.Push(ctx, true)
	case scenario.Pull:
		return c.Pull(ctx, step.Limit)
	case scenario.PullAll:
		return c.PullAll(ctx)
	case scenario.Offline:
		return c.Disconnect(), nil
	case scenario.Online:
		return c.Connect(ctx)
	case scenario.React:
		return c.React(ctx)
	case scenario.StaleHash:
		return c.Corrupt(sim.WrongHash), nil
	case scenario.StaleAhead:
		return c.Corrupt(sim.AheadOfHead), nil
	default:
		return "", fmt.Errorf("unknown step %s", step.Op)
	}
}

func (e *execution) bumpEpoch(ctx context.Context) (string, error) {
	if e.cfg.Epochs == nil {
		return "no database access; skipped", nil
	}
	epoch, err := e.cfg.Epochs.Bump(ctx, e.scope)
	if errors.Is(err, ErrNoScope) {
		return "scope has no log yet; skipped", nil
	}
	if err != nil {
		return "", err
	}
	for _, c := range e.clients {
		c.EpochBumped(epoch)
	}
	return fmt.Sprintf("epoch is now %d", epoch), nil
}

func (e *execution) log(format string, args ...any) {
	e.trace = append(e.trace, fmt.Sprintf(format, args...))
}

func (e *execution) close() {
	for _, c := range e.clients {
		c.Close()
	}
}

func nonce() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
