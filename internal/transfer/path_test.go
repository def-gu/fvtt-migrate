package transfer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeJoinRejectsEscapes(t *testing.T) {
	root := t.TempDir()

	for _, rel := range []string{
		"",
		"/etc/passwd",
		"../../.ssh/authorized_keys",
		"a/../../b",
		`..\..\windows\system32`,
		`C:\Windows\System32\drivers\etc\hosts`,
		"a//b",
		"./a",
		"a/./b",
		"nul\x00byte",
	} {
		if _, err := safeJoin(root, rel); !errors.Is(err, ErrUnsafePath) {
			t.Errorf("safeJoin(%q) = %v, want ErrUnsafePath", rel, err)
		}
	}
}

func TestSafeJoinAcceptsOrdinaryPaths(t *testing.T) {
	root := t.TempDir()

	for _, rel := range []string{
		"modules/lib-wrapper/module.json",
		"Карты/схема.webp",
		`modules\dfreds\tiles\x.webp`,
		"worlds/w/data/actors/000001.ldb",
	} {
		got, err := safeJoin(root, rel)
		if err != nil {
			t.Errorf("safeJoin(%q): %v", rel, err)
			continue
		}
		if !strings.HasPrefix(got, root+string(os.PathSeparator)) {
			t.Errorf("safeJoin(%q) = %q, outside root", rel, got)
		}
	}
}

func TestSafeJoinRejectsSymlinkTarget(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "innocent.webp")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := safeJoin(root, "innocent.webp"); !errors.Is(err, ErrUnsafePath) {
		t.Errorf("writing through an existing symlink was allowed: %v", err)
	}
}
