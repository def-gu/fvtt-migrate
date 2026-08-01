package transfer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrUnsafePath = errors.New("unsafe path")

// A target hands us paths it chose. Joining them naively lets it write anywhere
// on the machine, so every component is checked before the join and the result
// is checked again after it.
func safeJoin(root, rel string) (string, error) {
	r := strings.ReplaceAll(rel, `\`, "/")
	switch {
	case r == "":
		return "", fmt.Errorf("%w: empty", ErrUnsafePath)
	case strings.HasPrefix(r, "/"):
		return "", fmt.Errorf("%w: absolute: %q", ErrUnsafePath, rel)
	case len(r) >= 2 && r[1] == ':':
		return "", fmt.Errorf("%w: drive letter: %q", ErrUnsafePath, rel)
	case strings.ContainsRune(r, 0):
		return "", fmt.Errorf("%w: NUL byte", ErrUnsafePath)
	}

	for _, part := range strings.Split(r, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("%w: %q", ErrUnsafePath, rel)
		}
	}

	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	abs := filepath.Join(cleanRoot, filepath.FromSlash(r))
	if !strings.HasPrefix(abs, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: escapes root: %q", ErrUnsafePath, rel)
	}

	if info, err := os.Lstat(abs); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%w: existing symlink: %q", ErrUnsafePath, rel)
	}
	return abs, nil
}
