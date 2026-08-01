package transfer

import (
	"sort"
	"strings"

	"github.com/def-gu/fvtt-migrate/internal/plan"
	"github.com/def-gu/fvtt-migrate/internal/scan"
)

type Selection struct {
	Paths []string
	Bytes int64
}

func Select(p *plan.Plan, ix *scan.Index, sum *scan.Summary) Selection {
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
	sort.Strings(sel.Paths)
	return sel
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
