package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
)

func TestClassification(t *testing.T) {
	data := t.TempDir()

	writeFile(t, data, "Карты/used.webp", "aaaa")
	writeFile(t, data, "Карты/unused.webm", "bbbbbbbbbb")
	writeFile(t, data, "modules/m/module.json", `{"id":"m","version":"1.0.0"}`)
	writeFile(t, data, "modules/m/art/never-referenced.png", "cc")
	writeFile(t, data, "stickers/lonely.png", "d")

	ix, err := Build(data, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	inv := &foundry.Inventory{
		Modules: []foundry.Package{{Kind: foundry.KindModule, ID: "m", Version: "1.0.0"}},
	}

	// Analyze walks worlds for references; with no worlds present the only
	// reference comes from the world background field, so drive it that way.
	inv.Worlds = []foundry.Package{{
		Kind:       foundry.KindWorld,
		ID:         "w",
		Dir:        emptyWorld(t, data),
		Background: "Карты/used.webp",
	}}

	s, err := Analyze(inv, ix, nil)
	if err != nil {
		t.Fatal(err)
	}

	if s.Referenced.Files != 1 || s.Referenced.Bytes != 4 {
		t.Errorf("referenced = %+v, want 1 file / 4 bytes", s.Referenced)
	}
	if s.Packaged.Files != 2 {
		t.Errorf("packaged = %+v, want 2 files (module.json and its art)", s.Packaged)
	}
	if s.Orphans.Files != 2 {
		t.Errorf("orphans = %+v, want 2 (unused.webm and lonely.png)", s.Orphans)
	}
	if got := s.OrphansByDir["Карты"]; got.Files != 1 || got.Bytes != 10 {
		t.Errorf("Карты orphans = %+v", got)
	}
}

func TestBrokenAndRenameCorrelation(t *testing.T) {
	data := t.TempDir()
	writeFile(t, data, "Карты/present.webm", "aaaa")

	ix, err := Build(data, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	inv := &foundry.Inventory{Worlds: []foundry.Package{{
		Kind:       foundry.KindWorld,
		ID:         "w",
		Dir:        emptyWorld(t, data),
		Background: "Карты/gone.webp",
	}}}

	s, err := Analyze(inv, ix, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(s.Broken) != 1 || s.Broken[0].Path != "Карты/gone.webp" {
		t.Fatalf("broken = %+v", s.Broken)
	}
	if renamed := s.Renamed(); len(renamed) != 1 || renamed[0] != "Карты" {
		t.Errorf("Renamed() = %v, want [Карты]", renamed)
	}
}

func TestRenameCorrelationIgnoresPackageNamespaces(t *testing.T) {
	data := t.TempDir()
	writeFile(t, data, "modules/stray.txt", "x")

	ix, err := Build(data, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	inv := &foundry.Inventory{Worlds: []foundry.Package{{
		Dir:        emptyWorld(t, data),
		Background: "modules/absent/a.webp",
	}}}

	s, err := Analyze(inv, ix, nil)
	if err != nil {
		t.Fatal(err)
	}
	if renamed := s.Renamed(); len(renamed) != 0 {
		t.Errorf("Renamed() = %v, want none: a missing module is not a rename", renamed)
	}
}

func emptyWorld(t *testing.T, data string) string {
	t.Helper()
	dir := filepath.Join(data, "worlds", "w")
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}
