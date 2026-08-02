package transfer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPlaceRefusesToFollowADirectorySymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	if err := os.Symlink(outside, filepath.Join(root, "modules")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	src, digests, sel := source(t, map[string]string{"modules/evil.txt": "escaped"})
	if _, err := Apply(context.Background(), src, sel, digests, NewFileTarget(root), Options{Workers: 1}); err != nil {
		t.Logf("apply refused: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outside, "evil.txt")); err == nil {
		t.Fatal("a write landed outside the root by way of a directory symlink")
	}
}
