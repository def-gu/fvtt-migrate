package transfer

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/content"
	"lukechampine.com/blake3"
)

func digestOf(b []byte) content.Digest {
	h := blake3.New(32, nil)
	h.Write(b)
	return content.Digest(hex.EncodeToString(h.Sum(nil)))
}

func TestPutThenPlace(t *testing.T) {
	ctx := context.Background()
	tgt := NewFileTarget(t.TempDir())
	body := []byte("a map, pretend it is large")
	d := digestOf(body)

	if err := tgt.Put(ctx, d, int64(len(body)), bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	if err := tgt.Place(ctx, d, "Карты/схема.webp"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(tgt.Root, "Карты", "схема.webp"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("placed %q, want %q", got, body)
	}
}

func TestPutRejectsWrongBytes(t *testing.T) {
	tgt := NewFileTarget(t.TempDir())
	claimed := digestOf([]byte("what was promised"))

	err := tgt.Put(context.Background(), claimed, -1, bytes.NewReader([]byte("what arrived")))
	if err == nil {
		t.Fatal("bytes that do not match the digest were stored")
	}
	if missing, _ := tgt.Missing(context.Background(), []content.Digest{claimed}); len(missing) != 1 {
		t.Error("rejected blob was left behind in the store")
	}
}

func TestPutRejectsWrongSize(t *testing.T) {
	tgt := NewFileTarget(t.TempDir())
	body := []byte("twelve bytes")

	if err := tgt.Put(context.Background(), digestOf(body), 999, bytes.NewReader(body)); err == nil {
		t.Error("size mismatch accepted")
	}
}

func TestMissingReportsOnlyAbsent(t *testing.T) {
	ctx := context.Background()
	tgt := NewFileTarget(t.TempDir())
	here := []byte("present")
	dHere, dGone := digestOf(here), digestOf([]byte("absent"))

	if err := tgt.Put(ctx, dHere, int64(len(here)), bytes.NewReader(here)); err != nil {
		t.Fatal(err)
	}

	missing, err := tgt.Missing(ctx, []content.Digest{dHere, dGone, dGone})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != dGone {
		t.Errorf("missing = %v, want exactly [%s]", missing, dGone)
	}
}

func TestPlaceRefusesToEscapeRoot(t *testing.T) {
	ctx := context.Background()
	tgt := NewFileTarget(t.TempDir())
	body := []byte("payload")
	d := digestOf(body)
	if err := tgt.Put(ctx, d, int64(len(body)), bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}

	if err := tgt.Place(ctx, d, "../../.ssh/authorized_keys"); !errors.Is(err, ErrUnsafePath) {
		t.Errorf("got %v, want ErrUnsafePath", err)
	}
}

func TestMalformedDigestIsRejected(t *testing.T) {
	tgt := NewFileTarget(t.TempDir())
	for _, d := range []content.Digest{"../../etc/passwd", "short", content.Digest(string(make([]byte, 64)))} {
		if _, err := tgt.Missing(context.Background(), []content.Digest{d}); !errors.Is(err, ErrUnsafePath) {
			t.Errorf("digest %q accepted", d)
		}
	}
}

func TestCommitClearsTheBlobStore(t *testing.T) {
	ctx := context.Background()
	tgt := NewFileTarget(t.TempDir())
	body := []byte("x")
	d := digestOf(body)

	if err := tgt.Put(ctx, d, 1, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	if err := tgt.Place(ctx, d, "a/b.webp"); err != nil {
		t.Fatal(err)
	}
	if err := tgt.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(tgt.Root, blobDir)); !os.IsNotExist(err) {
		t.Error("blob store survived commit")
	}
	if _, err := os.Stat(filepath.Join(tgt.Root, "a", "b.webp")); err != nil {
		t.Errorf("placed file did not survive commit: %v", err)
	}
}
