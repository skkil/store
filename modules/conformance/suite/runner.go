package suite

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/skkil/store/modules/conformance/suite/check"
	"github.com/skkil/store/modules/conformance/suite/scenario"
	"github.com/skkil/store/modules/conformance/suite/sim"
)

// Run runs every configured scenario for every seed and reports each run to
// cfg.Out. It returns an error only when the configuration is invalid; a
// failing run is in the Report.
func Run(ctx context.Context, cfg Config) (*Report, error) {
	cfg, err := cfg.withDefaults()
	if err != nil {
		return nil, err
	}
	opts := scenario.Options{Clients: cfg.Clients, Steps: cfg.Steps, EpochBumps: cfg.Epochs != nil}
	for _, name := range cfg.Scenarios {
		if _, err := scenario.Build(name, cfg.Seed, opts); err != nil {
			return nil, err
		}
	}
	transport := sim.NewTransport(cfg.HTTPClient, cfg.PushURL, cfg.StoreURL)
	report := &Report{}
	for run := range cfg.Runs {
		seed := cfg.Seed + uint64(run)
		for _, name := range cfg.Scenarios {
			if ctx.Err() != nil {
				return report, nil
			}
			sc, _ := scenario.Build(name, seed, opts)
			result := execute(ctx, cfg, transport, sc, seed)
			if result.Failed() && result.Err == nil && cfg.Shrink {
				result = shrink(ctx, cfg, transport, sc, seed, result)
			}
			printResult(cfg, result)
			report.Results = append(report.Results, result)
		}
	}
	summarize(cfg, report)
	return report, nil
}

func printResult(cfg Config, r Result) {
	if !r.Failed() {
		fmt.Fprintf(cfg.Out, "PASS %-12s seed=%d steps=%d %s\n", r.Scenario, r.Seed, r.Steps, r.Duration.Round(1e6))
		if cfg.Verbose {
			printTrace(cfg, r)
		}
		return
	}
	fmt.Fprintf(cfg.Out, "FAIL %-12s seed=%d steps=%d %s\n", r.Scenario, r.Seed, r.Steps, r.Duration.Round(1e6))
	if r.Err != nil {
		fmt.Fprintf(cfg.Out, "  error: %v\n", r.Err)
	}
	for _, v := range r.Violations {
		fmt.Fprintf(cfg.Out, "  %s\n", v)
	}
	fmt.Fprintf(cfg.Out, "  reproduce: -seed %d -runs 1 -scenarios %s\n", r.Seed, r.Scenario)
	printTrace(cfg, r)
}

func printTrace(cfg Config, r Result) {
	fmt.Fprintln(cfg.Out, "  trace:")
	for _, line := range r.Trace {
		fmt.Fprintf(cfg.Out, "    %s\n", line)
	}
}

func summarize(cfg Config, report *Report) {
	failed := 0
	for _, r := range report.Results {
		if r.Failed() {
			failed++
		}
	}
	var broken []string
	for invariant := range report.Violated() {
		broken = append(broken, string(invariant))
	}
	slices.Sort(broken)
	if failed == 0 {
		fmt.Fprintf(cfg.Out, "ok: %d runs passed\n", len(report.Results))
		return
	}
	fmt.Fprintf(cfg.Out, "FAILED: %d of %d runs; broken: %s\n", failed, len(report.Results), strings.Join(broken, ", "))
}

func invariantsOf(violations []check.Violation) map[check.Invariant]bool {
	set := map[check.Invariant]bool{}
	for _, v := range violations {
		set[v.Invariant] = true
	}
	return set
}
