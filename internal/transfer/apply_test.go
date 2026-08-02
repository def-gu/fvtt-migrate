package transfer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/content"
)

func source(t *testing.T, files map[string]string) (string, map[string]content.Entry, Selection) {
	t.Helper()
	root := t.TempDir()

	var sel Selection
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		sel.Paths = append(sel.Paths, rel)
		sel.Bytes += int64(len(body))
	}

	res := content.HashTree(root, sel.Paths, content.OpenCache(root+"-nocache"), 2)
	if len(res.Errors) != 0 {
		t.Fatalf("hashing: %v", res.Errors)
	}
	return root, res.Entries, sel
}

func TestApplyTransfersEverythingOnce(t *testing.T) {
	ctx := context.Background()
	root, digests, sel := source(t, map[string]string{
		"Карты/one.webp":         "map one",
		"Карты/duplicate.webp":   "map one",
		"modules/m/module.json":  `{"id":"m"}`,
		"worlds/w/data/x/00.ldb": "leveldb bytes",
	})

	tgt := NewFileTarget(t.TempDir())
	prog, err := Apply(ctx, root, sel, digests, tgt, Options{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}

	if prog.Selected != 4 {
		t.Errorf("selected = %d, want 4", prog.Selected)
	}
	if prog.Negotiated != 3 {
		t.Errorf("negotiated = %d unique digests, want 3: the duplicate must collapse", prog.Negotiated)
	}
	if prog.Uploaded != 3 || prog.Placed != 4 {
		t.Errorf("uploaded=%d placed=%d, want 3 and 4", prog.Uploaded, prog.Placed)
	}

	for rel, want := range map[string]string{
		"Карты/one.webp":       "map one",
		"Карты/duplicate.webp": "map one",
	} {
		got, err := os.ReadFile(filepath.Join(tgt.Root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
}

func TestApplyResumesAfterAnInterruptedRun(t *testing.T) {
	ctx := context.Background()
	root, digests, sel := source(t, map[string]string{
		"a.webp": "first",
		"b.webp": "second",
	})

	dir := t.TempDir()
	interrupted := NewFileTarget(dir)
	e := digests["a.webp"]
	if err := interrupted.Put(ctx, e.Digest, e.Size, mustOpen(t, root, "a.webp")); err != nil {
		t.Fatal(err)
	}

	prog, err := Apply(ctx, root, sel, digests, NewFileTarget(dir), Options{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if prog.Uploaded != 1 || prog.Skipped != 1 {
		t.Errorf("uploaded=%d skipped=%d, want 1 and 1: the blob left behind must be reused",
			prog.Uploaded, prog.Skipped)
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	ctx := context.Background()
	root, digests, sel := source(t, map[string]string{
		"a.webp": "first",
		"b.webp": "second",
	})

	dir := t.TempDir()
	first := NewFileTarget(dir)
	if _, err := Apply(ctx, root, sel, digests, first, Options{Workers: 1}); err != nil {
		t.Fatal(err)
	}

	second := NewFileTarget(dir)
	prog, err := Apply(ctx, root, sel, digests, second, Options{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if prog.Uploaded != 2 {
		t.Errorf("uploaded=%d after Commit cleared the store; a re-run must repopulate it", prog.Uploaded)
	}
	if prog.Placed != 2 {
		t.Errorf("placed = %d, want 2", prog.Placed)
	}
}

func TestApplySkipsBlobsTheTargetAlreadyHas(t *testing.T) {
	ctx := context.Background()
	root, digests, sel := source(t, map[string]string{"a.webp": "shared", "b.webp": "unique"})

	tgt := NewFileTarget(t.TempDir())
	if err := tgt.Put(ctx, digests["a.webp"].Digest, digests["a.webp"].Size, mustOpen(t, root, "a.webp")); err != nil {
		t.Fatal(err)
	}

	prog, err := Apply(ctx, root, sel, digests, tgt, Options{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if prog.Skipped != 1 || prog.Uploaded != 1 {
		t.Errorf("skipped=%d uploaded=%d, want 1 and 1", prog.Skipped, prog.Uploaded)
	}
	if prog.SkippedByte != digests["a.webp"].Size {
		t.Errorf("skipped bytes = %d, want %d", prog.SkippedByte, digests["a.webp"].Size)
	}
}

func mustOpen(t *testing.T, root, rel string) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// A target that only offers Place must still be laid out correctly, and a
// target that batches must produce the same tree in far fewer calls.
type countingTarget struct {
	*FileTarget
	single, batches int
	batching        bool
}

func (c *countingTarget) Place(ctx context.Context, d content.Digest, rel string) error {
	c.single++
	return c.FileTarget.Place(ctx, d, rel)
}

func (c *countingTarget) PlaceMany(ctx context.Context, entries []content.Placement) error {
	if !c.batching {
		return errors.New("this target does not batch")
	}
	c.batches++
	for _, e := range entries {
		if err := c.FileTarget.Place(ctx, e.Digest, e.Path); err != nil {
			return err
		}
	}
	return nil
}

func TestApplyBatchesPlacementWhenTheTargetCan(t *testing.T) {
	ctx := context.Background()
	files := map[string]string{}
	for i := 0; i < 2100; i++ {
		files[fmt.Sprintf("Карты/%04d.webp", i)] = fmt.Sprintf("tile %d", i)
	}
	root, digests, sel := source(t, files)

	batching := &countingTarget{FileTarget: NewFileTarget(t.TempDir()), batching: true}
	if _, err := Apply(ctx, root, sel, digests, batching, Options{Workers: 2}); err != nil {
		t.Fatal(err)
	}
	if batching.single != 0 {
		t.Errorf("a batching target still received %d single calls", batching.single)
	}
	if batching.batches != 2 {
		t.Errorf("batches = %d, want 2 for 2100 files", batching.batches)
	}

	plain := NewFileTarget(t.TempDir())
	if _, err := Apply(ctx, root, sel, digests, plain, Options{Workers: 2}); err != nil {
		t.Fatal(err)
	}

	for rel := range files {
		batched, err := os.ReadFile(filepath.Join(batching.Root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s missing after a batched run: %v", rel, err)
		}
		one, err := os.ReadFile(filepath.Join(plain.Root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if string(batched) != string(one) {
			t.Fatalf("%s differs between a batched and a one by one run", rel)
		}
	}
}
