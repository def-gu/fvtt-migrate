package transfer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReceiverSkipsWhatItAlreadyPlaced(t *testing.T) {
	ctx := context.Background()
	root, digests, sel := source(t, map[string]string{
		"a.webp": "first",
		"b.webp": "second",
	})
	dir := t.TempDir()

	first, err := Apply(ctx, root, sel, digests, NewReceiver(dir), Options{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if first.Uploaded != 2 {
		t.Fatalf("first push uploaded %d, want 2", first.Uploaded)
	}

	second, err := Apply(ctx, root, sel, digests, NewReceiver(dir), Options{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if second.Uploaded != 0 || second.Skipped != 2 {
		t.Errorf("second push uploaded=%d skipped=%d, want 0 and 2", second.Uploaded, second.Skipped)
	}
	if second.Placed != 2 {
		t.Errorf("placed = %d, want 2", second.Placed)
	}
}

func TestReceiverAsksAgainWhenItsTreeChanged(t *testing.T) {
	ctx := context.Background()
	root, digests, sel := source(t, map[string]string{"a.webp": "payload"})
	dir := t.TempDir()

	if _, err := Apply(ctx, root, sel, digests, NewReceiver(dir), Options{Workers: 1}); err != nil {
		t.Fatal(err)
	}

	placed := filepath.Join(dir, "a.webp")
	if err := os.WriteFile(placed, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(placed, future, future); err != nil {
		t.Fatal(err)
	}

	again, err := Apply(ctx, root, sel, digests, NewReceiver(dir), Options{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if again.Uploaded != 1 {
		t.Errorf("uploaded = %d, want 1: a receiver whose tree was edited must not trust itself", again.Uploaded)
	}

	got, err := os.ReadFile(placed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Errorf("placed file = %q, want the source bytes back", got)
	}
}

func TestPlainTargetDoesNotRemember(t *testing.T) {
	ctx := context.Background()
	root, digests, sel := source(t, map[string]string{"a.webp": "x"})
	dir := t.TempDir()

	if _, err := Apply(ctx, root, sel, digests, NewFileTarget(dir), Options{Workers: 1}); err != nil {
		t.Fatal(err)
	}
	again, err := Apply(ctx, root, sel, digests, NewFileTarget(dir), Options{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if again.Uploaded != 1 {
		t.Errorf("uploaded = %d, want 1: a one-shot copy keeps no index", again.Uploaded)
	}
}
