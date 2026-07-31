package foundry

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func newInstall(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"Data", "Config"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestOpenAcceptsRootAndDataDir(t *testing.T) {
	root := newInstall(t)

	for _, arg := range []string{root, filepath.Join(root, "Data")} {
		inst, err := Open(arg)
		if err != nil {
			t.Fatalf("Open(%q): %v", arg, err)
		}
		if inst.Root != root {
			t.Errorf("Open(%q).Root = %q, want %q", arg, inst.Root, root)
		}
	}
}

func TestOpenRejectsUnrelatedDir(t *testing.T) {
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNotFoundry) {
		t.Errorf("got %v, want ErrNotFoundry", err)
	}
}

func TestOptions(t *testing.T) {
	root := newInstall(t)
	body := `{"dataPath":"/srv/fvtt","language":"ru.ru-ru","port":30000}`
	if err := os.WriteFile(filepath.Join(root, "Config", "options.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	inst, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := inst.Options()
	if err != nil {
		t.Fatal(err)
	}
	if opts.DataPath != "/srv/fvtt" || opts.Port != 30000 {
		t.Errorf("got %+v", opts)
	}
}
