package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An empty Go slice encodes as null, which is not a list and crashes a reader
// that iterates it. Every list the panel receives is checked here rather than
// once per endpoint, because this has now broken the interface three times.
func TestEmptyListsEncodeAsLists(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{"detect", Detect()},
		{"browse entries", &Listing{Entries: []Entry{}, Roots: []string{}}},
	}

	for _, c := range cases {
		raw, err := json.Marshal(c.value)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if strings.Contains(string(raw), "null") {
			t.Errorf("%s encodes a null where the panel expects a list: %s", c.name, raw)
		}
	}
}

func TestBrowseMarksFoundryDirectories(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"install/Data/worlds", "install/Config", "elsewhere"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	list, err := Browse(root)
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]bool{}
	for _, e := range list.Entries {
		found[e.Name] = e.Foundry
	}
	if !found["install"] {
		t.Error("a directory holding Data and Config was not marked as an installation")
	}
	if found["elsewhere"] {
		t.Error("an unrelated directory was marked as an installation")
	}
	if list.Parent == "" {
		t.Error("no parent offered, so the picker cannot go up")
	}
}

func TestBrowseRefusesAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Browse(file); err == nil {
		t.Error("browsing a file succeeded, want an error")
	}
}
