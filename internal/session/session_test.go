package session

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/progress"
	"github.com/syndtr/goleveldb/leveldb"
)

func newInstall(t *testing.T, world string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range []struct{ rel, body string }{
		{"Config/options.json", `{}`},
		{"Data/systems/pf2e/system.json", `{"id":"pf2e","title":"Pathfinder","version":"7.0.0"}`},
		{"Data/worlds/" + world + "/world.json", `{"id":"` + world + `","title":"Мир","system":"pf2e","coreVersion":"13.351"}`},
	} {
		p := filepath.Join(root, filepath.FromSlash(f.rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	db, err := leveldb.OpenFile(filepath.Join(root, "Data", "worlds", world, "data", "actors"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put([]byte("!actors!one"), []byte(`{"img":"worlds/`+world+`/hero.webp"}`), nil); err != nil {
		t.Fatal(err)
	}
	db.Close()
	return root
}

func TestSnapshotIsReusedUntilRootChanges(t *testing.T) {
	first := newInstall(t, "azlant")
	second := newInstall(t, "germes")
	s := New("")

	if _, err := s.Snapshot(); !errors.Is(err, ErrNoInstallation) {
		t.Errorf("snapshot before opening = %v, want ErrNoInstallation", err)
	}
	if _, err := s.Open(first); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Snapshot(); !errors.Is(err, ErrNotScanned) {
		t.Errorf("snapshot before reading = %v, want ErrNotScanned", err)
	}

	read, err := s.Read(nil)
	if err != nil {
		t.Fatal(err)
	}
	cached, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if cached != read {
		t.Error("a second request rebuilt the snapshot instead of answering from it")
	}

	if _, err := s.Open(second); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Snapshot(); !errors.Is(err, ErrNotScanned) {
		t.Error("a snapshot of one installation was kept after opening another")
	}
}

func TestReadReportsProgress(t *testing.T) {
	s := New("")
	if _, err := s.Open(newInstall(t, "azlant")); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	phases := map[string]bool{}
	sink := progress.Func(func(v any) {
		if e, ok := v.(progress.Event); ok {
			mu.Lock()
			phases[string(e.Phase)] = true
			mu.Unlock()
		}
	})
	if _, err := s.Read(sink); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"packages", "indexing", "worlds", "classifying"} {
		if !phases[want] {
			t.Errorf("phase %q was never reported", want)
		}
	}
}
