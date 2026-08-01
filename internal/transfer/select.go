package transfer

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/def-gu/fvtt-migrate/internal/plan"
	"github.com/def-gu/fvtt-migrate/internal/scan"
)

type Selection struct {
	Paths []string
	Bytes int64
}

func Select(p *plan.Plan, dataRoot string, ix *scan.Index, sum *scan.Summary) (Selection, error) {
	worlds := map[string]bool{}
	for _, w := range p.Worlds {
		if w.Include {
			worlds[w.ID] = true
		}
	}

	// Packages the target fetches for itself never travel from here.
	upload := map[string]bool{}
	for _, pkg := range p.Packages {
		if pkg.Source == plan.FromUpload {
			upload[pkg.Kind+"s/"+pkg.ID] = true
		}
	}

	dirs := map[string]bool{}
	for _, d := range p.Assets.Directories {
		dirs[d.Path] = d.Action == "include"
	}

	var sel Selection
	ix.Each(func(rel string, size int64) {
		if !include(rel, worlds, upload, dirs, sum) {
			return
		}
		sel.Paths = append(sel.Paths, rel)
		sel.Bytes += size
	})

	// The asset index deliberately omits world databases, which hold documents
	// rather than assets. They still have to travel, or the target gets worlds
	// with no contents at all.
	for id := range worlds {
		if err := appendWorldDatabase(dataRoot, id, &sel); err != nil {
			return sel, err
		}
	}

	sort.Strings(sel.Paths)
	return sel, nil
}

func appendWorldDatabase(dataRoot, worldID string, sel *Selection) error {
	base := filepath.Join(dataRoot, "worlds", worldID, "data")
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil
	}

	return filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dataRoot, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		sel.Paths = append(sel.Paths, filepath.ToSlash(rel))
		sel.Bytes += info.Size()
		return nil
	})
}

func include(rel string, worlds, upload map[string]bool, dirs map[string]bool, sum *scan.Summary) bool {
	parts := strings.SplitN(rel, "/", 3)
	top := parts[0]

	switch top {
	case "worlds":
		return len(parts) >= 2 && worlds[parts[1]]
	case "modules", "systems":
		return len(parts) >= 2 && upload[top+"/"+parts[1]]
	}

	if sum.Classify(rel) == scan.Referenced {
		return true
	}
	return dirs[top]
}
