package content

import (
	"os"
	"testing"
	"time"
)

func TestRecheckDetectsChangeUnderneath(t *testing.T) {
	root := t.TempDir()
	write(t, root, "steady.bin", "unchanged")
	moving := write(t, root, "moving.bin", "before")
	vanishing := write(t, root, "vanishing.bin", "here")

	rels := []string{"steady.bin", "moving.bin", "vanishing.bin"}
	entries := HashTree(root, rels, &Cache{entries: map[string]Entry{}}, 2).Entries

	if got := Recheck(root, entries); len(got) != 0 {
		t.Fatalf("a still tree reported changes: %v", got)
	}

	if err := os.WriteFile(moving, []byte("after!"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(moving, future, future); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(vanishing); err != nil {
		t.Fatal(err)
	}

	got := Recheck(root, entries)
	if len(got) != 2 || got[0] != "moving.bin" || got[1] != "vanishing.bin" {
		t.Errorf("Recheck = %v, want [moving.bin vanishing.bin]", got)
	}
}

func TestRecheckNoticesSameSizeRewrite(t *testing.T) {
	root := t.TempDir()
	p := write(t, root, "db.ldb", "aaaa")
	entries := HashTree(root, []string{"db.ldb"}, &Cache{entries: map[string]Entry{}}, 1).Entries

	if err := os.WriteFile(p, []byte("bbbb"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(p, future, future); err != nil {
		t.Fatal(err)
	}

	if got := Recheck(root, entries); len(got) != 1 {
		t.Errorf("an in-place rewrite of the same length went unnoticed: %v", got)
	}
}
