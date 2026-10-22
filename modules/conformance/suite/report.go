package suite

import (
	"time"

	"github.com/skkil/store/modules/conformance/suite/check"
)

// Result is one scenario run with one seed.
type Result struct {
	Scenario   string
	Seed       uint64
	Steps      int
	Duration   time.Duration
	Violations []check.Violation
	// Trace is the steps and what each returned; after shrinking, the
	// minimal failing run's trace.
	Trace []string
	// Err is a failure to run the scenario at all (bad credentials, an
	// unreachable server), as opposed to a violation.
	Err error
}

// Failed reports whether the run broke an invariant or could not run.
func (r Result) Failed() bool {
	return r.Err != nil || len(r.Violations) > 0
}

// Report is every scenario run.
type Report struct {
	Results []Result
}

// Failed reports whether any run failed.
func (r *Report) Failed() bool {
	for _, result := range r.Results {
		if result.Failed() {
			return true
		}
	}
	return false
}

// Violated returns the invariants broken in any run.
func (r *Report) Violated() map[check.Invariant]bool {
	broken := map[check.Invariant]bool{}
	for _, result := range r.Results {
		for _, v := range result.Violations {
			broken[v.Invariant] = true
		}
	}
	return broken
}
