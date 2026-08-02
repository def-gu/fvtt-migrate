package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIndexLookup(t *testing.T) {
	data := t.TempDir()
	core := t.TempDir()

	writeFile(t, data, "Карты/схема.webp", "12345")
	writeFile(t, data, "modules/m/a.png", "ab")
	writeFile(t, data, "worlds/w/data/actors/000001.ldb", "should be skipped")
	writeFile(t, data, "worlds/w/assets/keep.webp", "xyz")
	writeFile(t, core, "icons/svg/mystery-man.svg", "core")

	ix, err := Build(data, core, nil)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		path string
		want Location
		size int64
	}{
		{"Карты/схема.webp", InData, 5},
		{"modules/m/a.png", InData, 2},
		{"worlds/w/assets/keep.webp", InData, 3},
		{"icons/svg/mystery-man.svg", InCore, 0},
		{"Карты/СХЕМА.webp", CaseMismatch, 5},
		{"modules/m/nope.png", NotFound, 0},
	}

	for _, c := range cases {
		loc, size := ix.Lookup(c.path)
		if loc != c.want || size != c.size {
			t.Errorf("Lookup(%q) = (%v, %d), want (%v, %d)", c.path, loc, size, c.want, c.size)
		}
	}
}

func TestIndexSkipsWorldDatabase(t *testing.T) {
	data := t.TempDir()
	writeFile(t, data, "worlds/w/data/actors/000001.ldb", "x")
	writeFile(t, data, "worlds/w/data/scenes/CURRENT", "x")

	ix, err := Build(data, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if ix.Len() != 0 {
		t.Errorf("indexed %d database files, want 0", ix.Len())
	}
}
