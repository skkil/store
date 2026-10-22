package suite

import (
	"context"
	"slices"

	"github.com/skkil/store/modules/conformance/suite/scenario"
	"github.com/skkil/store/modules/conformance/suite/sim"
)

// shrink looks for a shorter run of sc that still breaks one of the
// invariants failed broke, by removing chunks of steps (halves first, then
// smaller), within cfg.ShrinkRuns re-runs. Each re-run uses a fresh scope.
func shrink(ctx context.Context, cfg Config, transport *sim.Transport, sc scenario.Scenario, seed uint64, failed Result) Result {
	target := invariantsOf(failed.Violations)
	best, steps := failed, sc.Steps
	runs := 0
	for chunk := len(steps) / 2; chunk >= 1 && runs < cfg.ShrinkRuns; {
		removed := false
		for start := 0; start < len(steps) && runs < cfg.ShrinkRuns; start += chunk {
			candidate := slices.Concat(steps[:start], steps[min(start+chunk, len(steps)):])
			if len(candidate) == 0 {
				continue
			}
			runs++
			result := execute(ctx, cfg, transport, sc.WithSteps(candidate), seed)
			if result.Err == nil && overlaps(invariantsOf(result.Violations), target) {
				best, steps, removed = result, candidate, true
				break
			}
		}
		if !removed {
			chunk /= 2
		}
	}
	best.Steps = len(steps)
	return best
}

func overlaps[K comparable](a, b map[K]bool) bool {
	for k := range a {
		if b[k] {
			return true
		}
	}
	return false
}
