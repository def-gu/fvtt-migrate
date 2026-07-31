package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Location int

const (
	NotFound Location = iota
	InData
	InCore
	// CaseMismatch means the file exists but under different capitalisation.
	// Such a reference works on Windows and breaks on a Linux server.
	CaseMismatch
)

type Index struct {
	data  map[string]int64
	lower map[string]string
	core  map[string]bool

	// Set when the Foundry application directory was not supplied, so that
	// references into it are recognised by prefix instead of by existence.
	coreByPrefix bool
}

// corePrefixes are the top-level directories Foundry ships inside its
// application bundle. Nothing under them is ever transferred.
var corePrefixes = map[string]bool{
	"icons": true, "sounds": true, "ui": true, "fonts": true,
	"cards": true, "canvas": true, "nue": true, "toolclips": true, "tours": true,
}

// skipDirs are never asset storage: world databases hold documents, and
// version-control metadata belongs to modules under development.
var skipDirs = map[string]bool{".git": true, "node_modules": true}

func Build(dataDir, coreDir string) (*Index, error) {
	ix := &Index{
		data:  make(map[string]int64),
		lower: make(map[string]string),
		core:  make(map[string]bool),
	}

	if err := walkFiles(dataDir, func(rel string, size int64) {
		ix.data[rel] = size
		ix.lower[strings.ToLower(rel)] = rel
	}); err != nil {
		return nil, err
	}

	if coreDir == "" {
		ix.coreByPrefix = true
		return ix, nil
	}
	if err := walkFiles(coreDir, func(rel string, _ int64) {
		ix.core[rel] = true
	}); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return ix, nil
}

func walkFiles(root string, visit func(rel string, size int64)) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] || isWorldDatabase(root, p) {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		visit(filepath.ToSlash(rel), info.Size())
		return nil
	})
}

func isWorldDatabase(root, dir string) bool {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	return len(parts) == 3 && parts[0] == "worlds" && parts[2] == "data"
}

func (ix *Index) Lookup(p string) (Location, int64) {
	if size, ok := ix.data[p]; ok {
		return InData, size
	}
	if ix.core[p] {
		return InCore, 0
	}
	if ix.coreByPrefix && corePrefixes[topSegment(p)] {
		return InCore, 0
	}
	if canonical, ok := ix.lower[strings.ToLower(p)]; ok {
		return CaseMismatch, ix.data[canonical]
	}
	return NotFound, 0
}

func topSegment(p string) string {
	if i := strings.Index(p, "/"); i >= 0 {
		return p[:i]
	}
	return p
}

func (ix *Index) Each(fn func(rel string, size int64)) {
	for rel, size := range ix.data {
		fn(rel, size)
	}
}

func (ix *Index) Len() int { return len(ix.data) }
