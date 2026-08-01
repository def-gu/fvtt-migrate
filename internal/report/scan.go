package report

import (
	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/scan"
)

type Counts struct {
	Worlds  int `json:"worlds"`
	Systems int `json:"systems"`
	Modules int `json:"modules"`
	Files   int `json:"files"`
}

type World struct {
	ID              string `json:"id"`
	System          string `json:"system"`
	SystemVersion   string `json:"system_version"`
	CoreVersion     string `json:"core_version"`
	SystemInstalled bool   `json:"system_installed"`
}

type Bucket struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

type Directory struct {
	Path   string `json:"path"`
	Files  int    `json:"files"`
	Bytes  int64  `json:"bytes"`
	Broken int    `json:"broken_references_into_it"`
	Stale  bool   `json:"looks_stale"`
}

type Assets struct {
	Referenced  Bucket      `json:"referenced"`
	Packaged    Bucket      `json:"in_packages"`
	Orphaned    Bucket      `json:"orphaned"`
	CoreRefs    int         `json:"built_in_references"`
	Directories []Directory `json:"orphan_directories"`
}

type Missing struct {
	Path  string `json:"path"`
	Refs  int    `json:"references"`
	Where string `json:"first_seen_at"`
}

type Problem struct {
	Dir    string `json:"dir"`
	Reason string `json:"reason"`
}

type Scan struct {
	Root     string    `json:"root"`
	Counts   Counts    `json:"counts"`
	Worlds   []World   `json:"worlds"`
	Assets   Assets    `json:"assets"`
	Broken   []Missing `json:"broken_references"`
	CaseOnly []Missing `json:"case_only_matches"`
	Problems []Problem `json:"unreadable_manifests"`
}

func BuildScan(inst *foundry.Install, inv *foundry.Inventory, ix *scan.Index, s *scan.Summary) *Scan {
	installed := map[string]bool{}
	for _, sys := range inv.Systems {
		installed[sys.ID] = true
	}

	out := &Scan{
		Root: inst.Root,
		Counts: Counts{
			Worlds:  len(inv.Worlds),
			Systems: len(inv.Systems),
			Modules: len(inv.Modules),
			Files:   ix.Len(),
		},
		Assets: Assets{
			Referenced: Bucket{s.Referenced.Files, s.Referenced.Bytes},
			Packaged:   Bucket{s.Packaged.Files, s.Packaged.Bytes},
			Orphaned:   Bucket{s.Orphans.Files, s.Orphans.Bytes},
			CoreRefs:   s.CoreRefs,
		},
	}

	for _, w := range inv.Worlds {
		out.Worlds = append(out.Worlds, World{
			ID:              w.ID,
			System:          w.System,
			SystemVersion:   w.SystemVersion,
			CoreVersion:     w.CoreVersion,
			SystemInstalled: installed[w.System],
		})
	}

	stale := map[string]bool{}
	for _, d := range s.Renamed() {
		stale[d] = true
	}
	for dir, b := range s.OrphansByDir {
		out.Assets.Directories = append(out.Assets.Directories, Directory{
			Path:   dir,
			Files:  b.Files,
			Bytes:  b.Bytes,
			Broken: s.BrokenByDir[dir],
			Stale:  stale[dir],
		})
	}
	sortDirs(out.Assets.Directories)

	out.Broken = convertMissing(s.Broken)
	out.CaseOnly = convertMissing(s.CaseIssues)
	for _, p := range inv.Problems {
		out.Problems = append(out.Problems, Problem{Dir: p.Dir, Reason: p.Reason})
	}
	return out
}

func convertMissing(in []scan.Missing) []Missing {
	out := make([]Missing, 0, len(in))
	for _, m := range in {
		out = append(out, Missing{Path: m.Path, Refs: m.Refs, Where: m.Where})
	}
	return out
}
