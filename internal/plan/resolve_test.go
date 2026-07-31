package plan

import (
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
)

type fakeCache map[string]bool

func (c fakeCache) Has(kind, id, v string) bool { return c[kind+"/"+id+"@"+v] }

func module(id, ver string, manifest *string, download string) foundry.Package {
	p := foundry.Package{Kind: foundry.KindModule, ID: id, Version: ver, Download: download}
	if manifest != nil {
		p.DeclaresManifest = true
		p.Manifest = *manifest
	}
	return p
}

func str(s string) *string { return &s }

func find(t *testing.T, list []Package, id string) Package {
	t.Helper()
	for _, p := range list {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("no package %q in plan", id)
	return Package{}
}

func TestResolveSources(t *testing.T) {
	inv := &foundry.Inventory{Modules: []foundry.Package{
		module("lib-wrapper", "1.13.5.1", str("https://x/releases/latest/download/module.json"),
			"https://x/releases/download/v1.13.5.1/lib-wrapper-v1.13.5.1.zip"),
		module("pf2e-kingmaker", "1.2.0", str(""), ""),
		module("globe-forge-spike", "0.1.0", nil, ""),
		module("stale-download", "2.0.0", str("https://x/module.json"),
			"https://x/releases/download/v1.0.0/stale-v1.0.0.zip"),
		module("cached", "3.1.0", str("https://x/module.json"), "https://x/v3.1.0/cached.zip"),
	}}

	got := ResolvePackages(inv, "13.351", fakeCache{"module/cached@3.1.0": true})

	cases := []struct {
		id      string
		source  Source
		premium bool
		shared  bool
	}{
		{"lib-wrapper", FromManifest, false, true},
		{"pf2e-kingmaker", FromUpload, true, false},
		{"globe-forge-spike", FromUpload, false, false},
		{"stale-download", FromUpload, false, true},
		{"cached", FromCache, false, true},
	}
	for _, c := range cases {
		p := find(t, got, c.id)
		if p.Source != c.source {
			t.Errorf("%s: source = %q, want %q (reason %q)", c.id, p.Source, c.source, p.Reason)
		}
		if p.Premium != c.premium {
			t.Errorf("%s: premium = %v, want %v", c.id, p.Premium, c.premium)
		}
		if p.Shared != c.shared {
			t.Errorf("%s: shared = %v, want %v", c.id, p.Shared, c.shared)
		}
	}
}

func TestPremiumIsNeverShared(t *testing.T) {
	inv := &foundry.Inventory{Modules: []foundry.Package{
		module("pf2e-kingmaker", "1.2.0", str(""), "https://paizo/kingmaker-1.2.0.zip"),
	}}

	p := find(t, ResolvePackages(inv, "13.351", fakeCache{"module/pf2e-kingmaker@1.2.0": true}), "pf2e-kingmaker")
	if p.Source != FromUpload {
		t.Errorf("premium resolved to %q; a cache hit must not serve paid content", p.Source)
	}
	if p.Shared {
		t.Error("premium package marked shareable")
	}
}

func TestDefaultPolicyIsPin(t *testing.T) {
	inv := &foundry.Inventory{Modules: []foundry.Package{module("m", "1.0.0", str("https://x"), "")}}
	if got := ResolvePackages(inv, "13.351", nil)[0].Policy; got != PolicyPin {
		t.Errorf("policy = %q, want %q", got, PolicyPin)
	}
}

func TestCompatVerdictFlows(t *testing.T) {
	m := module("old", "1.0.0", str("https://x"), "")
	m.Compat = foundry.Compatibility{Minimum: "9", Verified: "10"}
	inv := &foundry.Inventory{Modules: []foundry.Package{m}}

	if got := ResolvePackages(inv, "13.351", nil)[0].Compat; got != "untested" {
		t.Errorf("compat = %q, want untested", got)
	}
}
