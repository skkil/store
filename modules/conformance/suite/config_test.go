package suite

import (
	"context"
	"testing"

	"github.com/skkil/store/modules/conformance/suite/credentials"
	"github.com/skkil/store/modules/conformance/suite/profile"
	"github.com/skkil/store/modules/conformance/suite/scenario"
)

func validConfig(t *testing.T) Config {
	t.Helper()
	p, err := profile.Parse([]byte(`{"mutations":[{"name":"a","kind":"create"},{"name":"b","kind":"reject"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	return Config{PushURL: "http://push", StoreURL: "http://store", Credentials: credentials.CommandSource{Command: "true"}, Profile: p}
}

func TestDefaultsFillTheOptionalFields(t *testing.T) {
	cfg, err := validConfig(t).withDefaults()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runs != DefaultRuns || cfg.Clients != DefaultClients || cfg.Steps != DefaultSteps || cfg.PageSize != DefaultPageSize ||
		cfg.PokeTimeout != DefaultPokeTimeout || cfg.ConvergeRounds != DefaultConvergeRounds || cfg.ShrinkRuns != DefaultShrinkRuns ||
		cfg.RunTimeout != DefaultRunTimeout || len(cfg.Scenarios) != len(scenario.Names()) || cfg.HTTPClient == nil || cfg.Out == nil {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestRunRefusesIncompleteConfigs(t *testing.T) {
	for name, edit := range map[string]func(*Config){
		"push url":         func(c *Config) { c.PushURL = "" },
		"store url":        func(c *Config) { c.StoreURL = "" },
		"credentials":      func(c *Config) { c.Credentials = nil },
		"profile":          func(c *Config) { c.Profile = nil },
		"unknown scenario": func(c *Config) { c.Scenarios = []string{"chaos"} },
	} {
		cfg := validConfig(t)
		edit(&cfg)
		if _, err := Run(context.Background(), cfg); err == nil {
			t.Errorf("%s: Run accepted the config", name)
		}
	}
}

func TestUnreachableServersFailTheRunWithAnError(t *testing.T) {
	cfg := validConfig(t)
	cfg.Credentials = credentials.CommandSource{Command: "exit 1"}
	cfg.Scenarios = []string{"offline"}
	report, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Failed() || report.Results[0].Err == nil {
		t.Fatalf("report = %+v", report.Results)
	}
}
