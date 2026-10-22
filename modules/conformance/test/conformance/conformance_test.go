// Package conformance runs the suite against the reference product and the
// store service on real Postgres, and proves each check catches a broken
// product.
package conformance

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/skkil/store/modules/conformance/internal/stacktest"
	"github.com/skkil/store/modules/conformance/reference"
	"github.com/skkil/store/modules/conformance/suite"
	"github.com/skkil/store/modules/conformance/suite/check"
	"github.com/skkil/store/modules/conformance/suite/credentials"
	"github.com/skkil/store/modules/conformance/suite/profile"
)

const defaultSeeds = 12

func seeds(t *testing.T) int {
	t.Helper()
	if raw := os.Getenv("CONFORMANCE_SEEDS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("CONFORMANCE_SEEDS: %v", err)
		}
		return n
	}
	if testing.Short() {
		return 2
	}
	return defaultSeeds
}

func config(t *testing.T, stack *stacktest.Stack, product *stacktest.Product, out *strings.Builder) suite.Config {
	t.Helper()
	p, err := profile.Parse(reference.Profile)
	if err != nil {
		t.Fatal(err)
	}
	return suite.Config{
		PushURL:     product.URL,
		StoreURL:    stack.Store.URL,
		Credentials: credentials.URLSource{URL: product.TokenURL},
		Profile:     p,
		Seed:        1,
		Epochs:      suite.PostgresEpochs{Pool: stack.DB.Pool},
		Out:         out,
	}
}

func TestReferenceConforms(t *testing.T) {
	stack := stacktest.NewStack(t)
	product := stack.Product(t, reference.FaultNone)
	var out strings.Builder
	cfg := config(t, stack, product, &out)
	cfg.Runs = seeds(t)
	report, err := suite.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if report.Failed() {
		t.Fatalf("the reference product fails the suite:\n%s", out.String())
	}
	if want := cfg.Runs * len(cfgScenarios()); len(report.Results) != want {
		t.Fatalf("ran %d scenarios, want %d", len(report.Results), want)
	}
}

func cfgScenarios() []string {
	return []string{"offline", "concurrent", "duplicates", "rejections", "poke-loss", "small-pulls", "reconnect", "random"}
}

func TestBrokenProductsFail(t *testing.T) {
	stack := stacktest.NewStack(t)
	cases := []struct {
		fault     reference.Fault
		scenarios []string
		want      check.Invariant
	}{
		{reference.FaultIgnoreDuplicates, []string{"duplicates"}, check.AtMostOnce},
		{reference.FaultSequenceVersions, []string{"duplicates", "concurrent"}, check.Dense},
		{reference.FaultDropRejections, []string{"rejections"}, check.DurableRejection},
	}
	for _, tc := range cases {
		t.Run(string(tc.fault), func(t *testing.T) {
			product := stack.Product(t, tc.fault)
			var out strings.Builder
			cfg := config(t, stack, product, &out)
			cfg.Runs = 2
			cfg.Scenarios = tc.scenarios
			cfg.Shrink = true
			cfg.ShrinkRuns = 8
			report, err := suite.Run(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			for _, result := range report.Results {
				if result.Err != nil {
					t.Fatalf("%s seed %d could not run: %v", result.Scenario, result.Seed, result.Err)
				}
				if !result.Failed() {
					t.Errorf("%s seed %d passed against %s:\n%s", result.Scenario, result.Seed, tc.fault, out.String())
				}
			}
			if !report.Violated()[tc.want] {
				t.Fatalf("%s did not break %s:\n%s", tc.fault, tc.want, out.String())
			}
			if !strings.Contains(out.String(), "reproduce: -seed") || !strings.Contains(out.String(), "trace:") {
				t.Fatalf("a failure must print its seed and trace:\n%s", out.String())
			}
		})
	}
}
