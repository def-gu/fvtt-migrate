package transfer

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/def-gu/fvtt-migrate/internal/content"
	"lukechampine.com/blake3"
)

const blobDir = ".fvtt-blobs"

type FileTarget struct {
	Root string

	mu    sync.Mutex
	index *content.Cache
	held  map[content.Digest]string
}

func NewFileTarget(root string) *FileTarget {
	return &FileTarget{Root: root, held: map[content.Digest]string{}}
}

// A receiver that keeps answering for the same directory remembers what it has
// already placed, so a repeated push does not resend files that are lying in
// the tree under their final names.
func NewReceiver(root string) *FileTarget {
	t := NewFileTarget(root)
	t.index = content.OpenCache("receiver:" + root)
	for _, e := range t.index.Entries() {
		t.held[e.Digest] = e.Path
	}
	return t
}

func (t *FileTarget) blobPath(d content.Digest) (string, error) {
	if len(d) != 64 {
		return "", fmt.Errorf("%w: digest %q", ErrUnsafePath, d)
	}
	if _, err := hex.DecodeString(string(d)); err != nil {
		return "", fmt.Errorf("%w: digest %q", ErrUnsafePath, d)
	}
	return filepath.Join(t.Root, blobDir, string(d[:2]), string(d)), nil
}

func (t *FileTarget) Missing(_ context.Context, want []content.Digest) ([]content.Digest, error) {
	out := []content.Digest{}
	seen := map[content.Digest]bool{}

	for _, d := range want {
		if seen[d] {
			continue
		}
		seen[d] = true

		p, err := t.blobPath(d)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(p); err == nil {
			continue
		}
		if t.placedAt(d) != "" {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// A remembered path only counts while the file there is untouched, so a receiver
// that had its tree edited asks for the bytes again instead of trusting itself.
func (t *FileTarget) placedAt(d content.Digest) string {
	t.mu.Lock()
	rel, ok := t.held[d]
	t.mu.Unlock()
	if !ok {
		return ""
	}

	abs, err := safeJoin(t.Root, rel)
	if err != nil {
		return ""
	}
	info, err := os.Stat(abs)
	if err != nil {
		return ""
	}
	if _, fresh := t.index.Lookup(rel, info.Size(), info.ModTime().UnixNano()); !fresh {
		return ""
	}
	return abs
}

func (t *FileTarget) Put(_ context.Context, d content.Digest, size int64, r io.Reader) error {
	final, err := t.blobPath(d)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(final), ".incoming-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	h := blake3.New(32, nil)
	written, err := io.Copy(io.MultiWriter(tmp, h), r)
	if err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if got := content.Digest(hex.EncodeToString(h.Sum(nil))); got != d {
		return fmt.Errorf("digest mismatch: declared %s, received %s", d, got)
	}
	if size >= 0 && written != size {
		return fmt.Errorf("size mismatch for %s: declared %d, received %d", d, size, written)
	}
	return os.Rename(tmp.Name(), final)
}

func (t *FileTarget) Place(_ context.Context, d content.Digest, rel string) error {
	dest, err := safeJoin(t.Root, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	source, err := t.blobPath(d)
	if err != nil {
		return err
	}
	if _, err := os.Stat(source); err != nil {
		if held := t.placedAt(d); held != "" {
			source = held
		} else {
			return fmt.Errorf("no bytes for %s", d)
		}
	}
	if source == dest {
		t.remember(d, rel, dest)
		return nil
	}

	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Link(source, dest); err != nil {
		if err := copyFile(source, dest); err != nil {
			return err
		}
	}
	t.remember(d, rel, dest)
	return nil
}

func (t *FileTarget) PlaceMany(ctx context.Context, entries []content.Placement) error {
	for _, e := range entries {
		if err := t.Place(ctx, e.Digest, e.Path); err != nil {
			return err
		}
	}
	return nil
}

func (t *FileTarget) remember(d content.Digest, rel, abs string) {
	if t.index == nil {
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		return
	}
	t.index.Store(content.Entry{Path: rel, Size: info.Size(), ModTime: info.ModTime().UnixNano(), Digest: d})

	t.mu.Lock()
	t.held[d] = rel
	t.mu.Unlock()
}

func (t *FileTarget) Commit(_ context.Context) error {
	if t.index != nil {
		if err := t.index.Save(); err != nil {
			return err
		}
	}
	return os.RemoveAll(filepath.Join(t.Root, blobDir))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.CreateTemp(filepath.Dir(dst), ".placing-*")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(out.Name(), dst)
}
