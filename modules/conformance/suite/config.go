// Package suite runs the store conformance suite: seeded scenarios of
// simulated clients against a product's Push and the store service's Pull
// and Pokes, over the wire, checking the invariants of ARCHITECTURE.md §17.
package suite

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/skkil/store/modules/conformance/suite/credentials"
	"github.com/skkil/store/modules/conformance/suite/profile"
	"github.com/skkil/store/modules/conformance/suite/scenario"
)

// Defaults for the optional Config fields.
const (
	DefaultRuns           = 1
	DefaultClients        = 3
	DefaultSteps          = 60
	DefaultPageSize       = 50
	DefaultPokeTimeout    = 10 * time.Second
	DefaultConvergeRounds = 10
	DefaultShrinkRuns     = 40
	DefaultRunTimeout     = 2 * time.Minute
)

// Config is one conformance run.
type Config struct {
	// PushURL is the product server's base URL for skkil.store.v1.PushService.
	PushURL string
	// StoreURL is the store service's base URL.
	StoreURL string
	// Credentials gives each simulated client its sync token and Push headers.
	Credentials credentials.Source
	// Profile describes the product's mutations.
	Profile *profile.Profile
	// Seed is the first seed; run i uses Seed+i.
	Seed uint64
	// Runs is how many seeds to run.
	Runs int
	// Scenarios to run; empty runs every scenario.
	Scenarios []string
	// Clients per scenario.
	Clients int
	// Steps per scenario, roughly.
	Steps int
	// PageSize is the limit of a client's full pull.
	PageSize uint32
	// Epochs bumps a scope's epoch; nil leaves epoch bumps out.
	Epochs EpochBumper
	// PokeTimeout is how long a listening client may wait to be poked up to the head.
	PokeTimeout time.Duration
	// ConvergeRounds bounds the push-and-pull rounds after the steps.
	ConvergeRounds int
	// RunTimeout bounds one scenario run, so a server that stops answering
	// fails the run instead of hanging the suite.
	RunTimeout time.Duration
	// Shrink re-runs a failing scenario with fewer steps to find a minimal trace.
	Shrink bool
	// ShrinkRuns bounds those re-runs.
	ShrinkRuns int
	// HTTPClient carries every request; nil uses a fresh client.
	HTTPClient *http.Client
	// Out receives the progress and failure report; nil discards it.
	Out io.Writer
	// Verbose prints the trace of passing scenarios too.
	Verbose bool
}

func (c Config) withDefaults() (Config, error) {
	switch {
	case c.PushURL == "":
		return c, errors.New("suite: PushURL is required")
	case c.StoreURL == "":
		return c, errors.New("suite: StoreURL is required")
	case c.Credentials == nil:
		return c, errors.New("suite: Credentials is required")
	case c.Profile == nil:
		return c, errors.New("suite: Profile is required")
	}
	if c.Runs <= 0 {
		c.Runs = DefaultRuns
	}
	if len(c.Scenarios) == 0 {
		c.Scenarios = scenario.Names()
	}
	if c.Clients <= 0 {
		c.Clients = DefaultClients
	}
	if c.Steps <= 0 {
		c.Steps = DefaultSteps
	}
	if c.PageSize == 0 {
		c.PageSize = DefaultPageSize
	}
	if c.PokeTimeout <= 0 {
		c.PokeTimeout = DefaultPokeTimeout
	}
	if c.ConvergeRounds <= 0 {
		c.ConvergeRounds = DefaultConvergeRounds
	}
	if c.RunTimeout <= 0 {
		c.RunTimeout = DefaultRunTimeout
	}
	if c.ShrinkRuns <= 0 {
		c.ShrinkRuns = DefaultShrinkRuns
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}
	if c.Out == nil {
		c.Out = io.Discard
	}
	c.Credentials = credentials.NewCache(c.Credentials)
	return c, nil
}
