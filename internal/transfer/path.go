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
// on the machine, so every component is checked before the join, the result is
// checked after it, and the path is then resolved on disk: a lexical check
// alone is defeated by a directory symlink pointing out of the root.
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
	if err := insideRoot(cleanRoot, abs); err != nil {
		return "", err
	}
	return abs, nil
}

func insideRoot(cleanRoot, abs string) error {
	realRoot, err := filepath.EvalSymlinks(cleanRoot)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve the destination: %v", ErrUnsafePath, err)
	}

	dir := filepath.Dir(abs)
	for {
		resolved, err := filepath.EvalSymlinks(dir)
		if err == nil {
			if resolved != realRoot && !strings.HasPrefix(resolved, realRoot+string(os.PathSeparator)) {
				return fmt.Errorf("%w: %q leads outside the destination through a link to %s",
					ErrUnsafePath, dir[len(cleanRoot):], resolved)
			}
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return fmt.Errorf("%w: cannot resolve %q", ErrUnsafePath, dir)
		}
		dir = parent
	}
}
