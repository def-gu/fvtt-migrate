package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/content"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFilesReportsMissingAndAltered(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	for rel, body := range map[string]string{
		"intact.webp":  "same on both sides",
		"altered.webp": "original bytes",
		"absent.webp":  "never arrived",
	} {
		write(t, source, rel, body)
	}
	write(t, target, "intact.webp", "same on both sides")
	write(t, target, "altered.webp", "different bytes")

	rels := []string{"intact.webp", "altered.webp", "absent.webp"}
	expected := content.HashTree(source, rels, content.OpenCache(t.TempDir()), 2).Entries

	res, err := Files(target, expected, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Missing) != 1 || res.Missing[0] != "absent.webp" {
		t.Errorf("missing = %v, want [absent.webp]", res.Missing)
	}
	if len(res.Mismatch) != 1 || res.Mismatch[0] != "altered.webp" {
		t.Errorf("mismatch = %v, want [altered.webp]", res.Mismatch)
	}
	if res.OK() {
		t.Error("OK() true despite a missing and an altered file")
	}
}

func TestFilesShallowSkipsHashing(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	write(t, source, "a.webp", "1234")
	write(t, target, "a.webp", "4321")

	expected := content.HashTree(source, []string{"a.webp"}, content.OpenCache(t.TempDir()), 1).Entries

	res, err := Files(target, expected, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Rehashed != 0 {
		t.Errorf("rehashed %d files in shallow mode", res.Rehashed)
	}
	if !res.OK() {
		t.Error("shallow mode reported a fault it cannot see: sizes match")
	}
}

func TestWorldsCatchesAnEmptyDatabase(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()

	// A world whose files were placed but whose database never travelled is the
	// exact failure a byte-level check calls a success.
	if err := os.MkdirAll(filepath.Join(source, "worlds", "w", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(target, "worlds", "w", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, source, "worlds/w/world.json", `{"id":"w"}`)
	write(t, target, "worlds/w/world.json", `{"id":"w"}`)

	got, err := Worlds(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("checked %d worlds, want 1", len(got))
	}
	if !got[0].OK() {
		t.Errorf("two empty worlds compared unequal: %+v", got[0])
	}
}

func TestWorldsIgnoresWorldsNotMigrated(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "worlds", "left-behind", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(target, "worlds"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := Worlds(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("checked %d worlds, want 0: the plan excluded it deliberately", len(got))
	}
}
