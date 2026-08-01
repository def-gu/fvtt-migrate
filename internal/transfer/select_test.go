package transfer

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/plan"
	"github.com/def-gu/fvtt-migrate/internal/scan"
)

func fixture(t *testing.T) (string, *scan.Index, *scan.Summary) {
	t.Helper()
	data := t.TempDir()

	for rel, body := range map[string]string{
		"worlds/keep/world.json":             `{"id":"keep","system":"pf2e"}`,
		"worlds/keep/data/actors/000001.ldb": "leveldb",
		"worlds/keep/data/actors/CURRENT":    "MANIFEST-000000",
		"worlds/keep/assets/local.webp":      "asset",
		"worlds/drop/world.json":             `{"id":"drop","system":"gone"}`,
		"worlds/drop/data/actors/000001.ldb": "leveldb",
		"modules/fetched/module.json":        `{"id":"fetched"}`,
		"modules/uploaded/module.json":       `{"id":"uploaded"}`,
		"Карты/kept.webp":                    "map",
		"junk/ignored.webp":                  "junk",
	} {
		p := filepath.Join(data, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ix, err := scan.Build(data, "")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := scan.Analyze(&foundry.Inventory{
		Modules: []foundry.Package{
			{Kind: foundry.KindModule, ID: "fetched"},
			{Kind: foundry.KindModule, ID: "uploaded"},
		},
	}, ix)
	if err != nil {
		t.Fatal(err)
	}
	return data, ix, sum
}

func testPlan() *plan.Plan {
	return &plan.Plan{
		Worlds: []plan.World{
			{ID: "keep", Include: true},
			{ID: "drop", Include: false},
		},
		Packages: []plan.Package{
			{ID: "fetched", Kind: "module", Source: plan.FromManifest},
			{ID: "uploaded", Kind: "module", Source: plan.FromUpload},
		},
		Assets: plan.Assets{Directories: []plan.Directory{
			{Path: "Карты", Action: "include"},
			{Path: "junk", Action: "skip"},
		}},
	}
}

func TestSelectCarriesWorldDatabases(t *testing.T) {
	data, ix, sum := fixture(t)

	sel, err := Select(testPlan(), data, ix, sum)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"worlds/keep/data/actors/000001.ldb",
		"worlds/keep/data/actors/CURRENT",
	} {
		if !slices.Contains(sel.Paths, want) {
			t.Errorf("%s was not selected: the world would arrive with no contents", want)
		}
	}
}

func TestSelectHonoursPlanDecisions(t *testing.T) {
	data, ix, sum := fixture(t)

	sel, err := Select(testPlan(), data, ix, sum)
	if err != nil {
		t.Fatal(err)
	}

	included := map[string]bool{}
	for _, p := range sel.Paths {
		included[p] = true
	}

	for _, want := range []string{
		"worlds/keep/world.json",
		"worlds/keep/assets/local.webp",
		"modules/uploaded/module.json",
		"Карты/kept.webp",
	} {
		if !included[want] {
			t.Errorf("%s missing from the selection", want)
		}
	}
	for _, unwanted := range []string{
		"worlds/drop/world.json",
		"worlds/drop/data/actors/000001.ldb",
		"modules/fetched/module.json",
		"junk/ignored.webp",
	} {
		if included[unwanted] {
			t.Errorf("%s was selected against the plan", unwanted)
		}
	}
}
