package plan

import (
	"bytes"
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/scan"
)

func testInputs() (*foundry.Install, *foundry.Inventory, *scan.Summary) {
	inst := &foundry.Install{Root: "/srv/fvtt"}
	inv := &foundry.Inventory{
		Systems: []foundry.Package{{Kind: foundry.KindSystem, ID: "pf2e", Version: "7.12.2"}},
		Worlds: []foundry.Package{
			{Kind: foundry.KindWorld, ID: "ok", System: "pf2e", SystemVersion: "7.12.2", CoreVersion: "13.351"},
			{Kind: foundry.KindWorld, ID: "orphan", System: "cyberpunk2020", SystemVersion: "1.1.0", CoreVersion: "13.351"},
		},
	}
	sum := &scan.Summary{
		Referenced:   scan.Bucket{Files: 10, Bytes: 1000},
		Packaged:     scan.Bucket{Files: 5, Bytes: 500},
		OrphansByDir: map[string]scan.Bucket{"Карты": {Files: 3, Bytes: 900}, "junk": {Files: 1, Bytes: 10}},
		BrokenByDir:  map[string]int{"Карты": 7},
		Broken:       []scan.Missing{{Path: "Карты/gone.webp", Refs: 7}},
	}
	return inst, inv, sum
}

func buildDefault() *Plan {
	inst, inv, sum := testInputs()
	return Build(inst, inv, sum, "", nil)
}

func TestBuildBlocksWorldWithMissingSystem(t *testing.T) {
	byID := map[string]World{}
	for _, w := range buildDefault().Worlds {
		byID[w.ID] = w
	}

	if !byID["ok"].Include {
		t.Error("world with an installed system was excluded")
	}
	if byID["orphan"].Include {
		t.Error("world with a missing system was included")
	}
	if byID["orphan"].Blocker == "" {
		t.Error("blocked world carries no explanation")
	}
}

func TestBuildDefaultsRenamedDirectoriesToInclude(t *testing.T) {
	byPath := map[string]Directory{}
	for _, d := range buildDefault().Assets.Directories {
		byPath[d.Path] = d
	}

	if got := byPath["Карты"]; got.Action != "include" {
		t.Errorf("Карты action = %q, want include: broken references point into it", got.Action)
	}
	if got := byPath["junk"]; got.Action != "skip" {
		t.Errorf("junk action = %q, want skip", got.Action)
	}
}

func TestTargetCoreDefaultsToHighestSeen(t *testing.T) {
	inst, inv, sum := testInputs()
	inv.Worlds[1].CoreVersion = "14.363"

	if got := Build(inst, inv, sum, "", nil).Source.TargetCore; got != "14.363" {
		t.Errorf("target core = %q, want 14.363", got)
	}
}

func TestRoundTrip(t *testing.T) {
	p := buildDefault()

	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		t.Fatal(err)
	}
	back, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if back.FormatVersion != p.FormatVersion || len(back.Worlds) != len(p.Worlds) {
		t.Fatalf("round trip lost data: %+v", back)
	}
	if back.Source.PackageMode != PolicyPin {
		t.Errorf("package policy = %q, want pin", back.Source.PackageMode)
	}
	if back.Assets.Referenced.Bytes != 1000 {
		t.Errorf("asset totals not preserved: %+v", back.Assets)
	}
}
