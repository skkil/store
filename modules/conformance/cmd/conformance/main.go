// Command conformance runs the store conformance suite against a product
// server's Push and a store service. See the module README for the flags and
// the mutation profile format.
package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skkil/store/modules/conformance/suite"
	"github.com/skkil/store/modules/conformance/suite/credentials"
	"github.com/skkil/store/modules/conformance/suite/profile"
)

const (
	exitFailed = 1
	exitUsage  = 2
)

type headerFlags []string

func (h *headerFlags) String() string     { return strings.Join(*h, ", ") }
func (h *headerFlags) Set(v string) error { *h = append(*h, v); return nil }

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	code, err := run(ctx, os.Args[1:], os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "conformance:", err)
	}
	os.Exit(code)
}

func run(ctx context.Context, args []string, out io.Writer) (int, error) {
	flags := flag.NewFlagSet("conformance", flag.ContinueOnError)
	pushURL := flags.String("push-url", "", "product server base URL serving skkil.store.v1.PushService (required)")
	storeURL := flags.String("store-url", "", "store service base URL (required)")
	tokenURL := flags.String("token-url", "", "URL that answers POST {\"user\",\"client_id\"} with a sync token")
	tokenCmd := flags.String("token-cmd", "", "shell command printing a sync token; gets $"+credentials.EnvUser+" and $"+credentials.EnvClientID)
	var tokenHeaders headerFlags
	flags.Var(&tokenHeaders, "token-header", "extra 'Name: value' header for -token-url (repeatable)")
	profilePath := flags.String("profile", "", "mutation profile JSON (required)")
	seed := flags.Uint64("seed", 0, "first seed (default: random, printed)")
	runs := flags.Int("runs", suite.DefaultRuns, "how many consecutive seeds to run")
	scenarios := flags.String("scenarios", "", "comma-separated scenarios (default: all)")
	clients := flags.Int("clients", suite.DefaultClients, "simulated clients per scenario")
	steps := flags.Int("steps", suite.DefaultSteps, "steps per scenario, roughly")
	pageSize := flags.Uint("page-size", suite.DefaultPageSize, "limit of a client's full pull")
	databaseURL := flags.String("database-url", "", "product Postgres URL; enables epoch-bump scenarios")
	pokeTimeout := flags.Duration("poke-timeout", suite.DefaultPokeTimeout, "how long a listening client may wait for a poke up to the head")
	shrink := flags.Bool("shrink", true, "re-run a failing scenario with fewer steps to print a minimal trace")
	shrinkRuns := flags.Int("shrink-runs", suite.DefaultShrinkRuns, "most re-runs while shrinking")
	verbose := flags.Bool("v", false, "print the trace of passing runs too")
	if err := flags.Parse(args); err != nil {
		return exitUsage, nil
	}
	if *pushURL == "" || *storeURL == "" || *profilePath == "" {
		return exitUsage, errors.New("-push-url, -store-url and -profile are required")
	}
	source, err := tokenSource(*tokenURL, *tokenCmd, tokenHeaders)
	if err != nil {
		return exitUsage, err
	}
	p, err := profile.Load(*profilePath)
	if err != nil {
		return exitUsage, err
	}
	if !isSet(flags, "seed") {
		*seed = randomSeed()
	}
	cfg := suite.Config{
		PushURL:     *pushURL,
		StoreURL:    *storeURL,
		Credentials: source,
		Profile:     p,
		Seed:        *seed,
		Runs:        *runs,
		Clients:     *clients,
		Steps:       *steps,
		PageSize:    uint32(*pageSize),
		PokeTimeout: *pokeTimeout,
		Shrink:      *shrink,
		ShrinkRuns:  *shrinkRuns,
		HTTPClient:  &http.Client{},
		Out:         out,
		Verbose:     *verbose,
	}
	if *scenarios != "" {
		cfg.Scenarios = strings.Split(*scenarios, ",")
	}
	if *databaseURL != "" {
		pool, err := pgxpool.New(ctx, *databaseURL)
		if err != nil {
			return exitUsage, fmt.Errorf("database: %w", err)
		}
		defer pool.Close()
		cfg.Epochs = suite.PostgresEpochs{Pool: pool}
	}
	fmt.Fprintf(out, "conformance: seed %d, %d runs, started %s\n", cfg.Seed, cfg.Runs, time.Now().Format(time.RFC3339))
	report, err := suite.Run(ctx, cfg)
	if err != nil {
		return exitUsage, err
	}
	if report.Failed() {
		return exitFailed, nil
	}
	return 0, nil
}

func tokenSource(url, command string, headers []string) (credentials.Source, error) {
	switch {
	case url != "" && command != "":
		return nil, errors.New("set only one of -token-url and -token-cmd")
	case url != "":
		h := http.Header{}
		for _, line := range headers {
			name, value, ok := strings.Cut(line, ":")
			if !ok {
				return nil, fmt.Errorf("-token-header %q is not 'Name: value'", line)
			}
			h.Add(strings.TrimSpace(name), strings.TrimSpace(value))
		}
		return credentials.URLSource{URL: url, Header: h}, nil
	case command != "":
		return credentials.CommandSource{Command: command}, nil
	default:
		return nil, errors.New("set -token-url or -token-cmd")
	}
}

func isSet(flags *flag.FlagSet, name string) bool {
	set := false
	flags.Visit(func(f *flag.Flag) { set = set || f.Name == name })
	return set
}

func randomSeed() uint64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return binary.BigEndian.Uint64(b[:]) >> 1
}
