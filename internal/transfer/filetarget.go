package transfer

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/def-gu/fvtt-migrate/internal/content"
	"lukechampine.com/blake3"
)

const blobDir = ".fvtt-blobs"

type FileTarget struct {
	Root string
}

func NewFileTarget(root string) *FileTarget {
	return &FileTarget{Root: root}
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
	var out []content.Digest
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
		if _, err := os.Stat(p); err != nil {
			out = append(out, d)
		}
	}
	return out, nil
}

// The digest is recomputed while writing, so a target that is fed the wrong
// bytes for a digest rejects them instead of storing them under that name.
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
	blob, err := t.blobPath(d)
	if err != nil {
		return err
	}
	dest, err := safeJoin(t.Root, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Link(blob, dest); err == nil {
		return nil
	}
	return copyFile(blob, dest)
}

func (t *FileTarget) Commit(_ context.Context) error {
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
