package scenario

import (
	"reflect"
	"testing"
)

func TestBuildIsDeterministicPerSeed(t *testing.T) {
	opts := Options{Clients: 3, Steps: 40, EpochBumps: true}
	for _, name := range Names() {
		a, err := Build(name, 7, opts)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := Build(name, 7, opts)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("%s: two builds with seed 7 differ", name)
		}
		c, _ := Build(name, 8, opts)
		if reflect.DeepEqual(a.Steps, c.Steps) {
			t.Fatalf("%s: seeds 7 and 8 give the same steps", name)
		}
	}
}

func TestBuildRespectsOptions(t *testing.T) {
	for _, name := range Names() {
		s, err := Build(name, 1, Options{Clients: 4, Steps: 30})
		if err != nil {
			t.Fatal(err)
		}
		if s.Clients != 4 || len(s.Listening) != 4 || len(s.Steps) < 30 {
			t.Fatalf("%s: %d clients, %d steps", name, s.Clients, len(s.Steps))
		}
		walk(s.Steps, func(step Step) {
			if step.Op == BumpEpoch {
				t.Fatalf("%s: epoch bump without EpochBumps", name)
			}
			if step.Op != Concurrent && step.Op != Settle && (step.Client < 0 || step.Client >= 4) {
				t.Fatalf("%s: step %s for client %d", name, step, step.Client)
			}
		})
		for _, step := range s.Steps {
			if step.Op != Concurrent {
				continue
			}
			seen := map[int]bool{}
			for _, g := range step.Group {
				if seen[g.Client] {
					t.Fatalf("%s: client %d twice in %s", name, g.Client, step)
				}
				seen[g.Client] = true
			}
		}
	}
}

func TestBuildFloorsTheSize(t *testing.T) {
	s, err := Build("random", 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Clients != 2 || len(s.Steps) < 10 {
		t.Fatalf("%d clients, %d steps", s.Clients, len(s.Steps))
	}
}

func TestScenariosCoverTheirFaults(t *testing.T) {
	want := map[string][]Op{
		"offline":     {Offline, Online},
		"concurrent":  {Concurrent},
		"duplicates":  {PushLost},
		"rejections":  {Mutate},
		"poke-loss":   {React, Settle},
		"small-pulls": {Pull},
		"reconnect":   {StaleHash, StaleAhead, BumpEpoch},
	}
	for name, ops := range want {
		s, _ := Build(name, 3, Options{Clients: 3, Steps: 200, EpochBumps: true})
		have := map[Op]bool{}
		walk(s.Steps, func(step Step) { have[step.Op] = true })
		for _, op := range ops {
			if !have[op] {
				t.Errorf("%s has no %s step", name, op)
			}
		}
	}
	s, _ := Build("poke-loss", 1, Options{Clients: 4})
	if !reflect.DeepEqual(s.Listening, []bool{true, false, true, false}) {
		t.Fatalf("poke-loss listening = %v", s.Listening)
	}
}

func TestBuildRefusesUnknownScenarios(t *testing.T) {
	if _, err := Build("chaos", 1, Options{}); err == nil {
		t.Fatal("unknown scenario built")
	}
}

func TestStepStrings(t *testing.T) {
	tests := map[string]Step{
		"c1 push":                       {Op: Push, Client: 1},
		"c0 pull limit=2":               {Op: Pull, Limit: 2},
		"c2 mutate reject":              {Op: Mutate, Client: 2, Kind: "reject"},
		"bump-epoch":                    {Op: BumpEpoch},
		"concurrent{c0 push | c1 push}": {Op: Concurrent, Group: []Step{{Op: Push}, {Op: Push, Client: 1}}},
	}
	for want, step := range tests {
		if got := step.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

func walk(steps []Step, visit func(Step)) {
	for _, s := range steps {
		visit(s)
		walk(s.Group, visit)
	}
}
