package report

import (
	"sort"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/scan"
)

type Size struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

type InventoryWorld struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Path            string   `json:"path"`
	System          string   `json:"system"`
	SystemTitle     string   `json:"system_title"`
	SystemVersion   string   `json:"system_version"`
	SystemInstalled bool     `json:"system_installed"`
	CoreVersion     string   `json:"core_version"`
	LastPlayed      string   `json:"last_played"`
	Size            Size     `json:"size"`
	ActiveModules   []string `json:"active_modules"`
	MissingModules  []string `json:"missing_modules"`
}

type InventoryPackage struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Version       string   `json:"version"`
	Path          string   `json:"path"`
	Authors       []string `json:"authors"`
	Delivery      string   `json:"delivery"`
	Manifest      string   `json:"manifest"`
	Verified      string   `json:"verified_core"`
	Minimum       string   `json:"minimum_core"`
	Maximum       string   `json:"maximum_core"`
	Size          Size     `json:"size"`
	TargetSystems []string `json:"target_systems"`
	Requires      []string `json:"requires"`
	Missing       []string `json:"missing_requirements"`
	UsedByWorlds  []string `json:"used_by_worlds"`
	ModuleCount   int      `json:"module_count,omitempty"`
}

type Inventory struct {
	Root    string             `json:"root"`
	Worlds  []InventoryWorld   `json:"worlds"`
	Systems []InventoryPackage `json:"systems"`
	Modules []InventoryPackage `json:"modules"`
}

func BuildInventory(inst *foundry.Install, inv *foundry.Inventory, ix *scan.Index) *Inventory {
	sizes := ix.Sizes(2)
	systems := map[string]foundry.Package{}
	for _, s := range inv.Systems {
		systems[s.ID] = s
	}
	installed := map[string]bool{}
	for _, p := range append(append([]foundry.Package{}, inv.Systems...), inv.Modules...) {
		installed[p.ID] = true
	}

	out := &Inventory{Root: inst.Root}
	enabledIn := map[string][]string{}
	worldsBySystem := map[string][]string{}

	for _, w := range inv.Worlds {
		active, _ := foundry.ActiveModules(w.Dir)
		var missing []string
		for _, id := range active {
			if !installed[id] {
				missing = append(missing, id)
			}
			enabledIn[id] = append(enabledIn[id], w.ID)
		}

		system, hasSystem := systems[w.System]
		worldsBySystem[w.System] = append(worldsBySystem[w.System], w.ID)

		size := sizeOf(sizes, "worlds/"+w.ID)
		files, bytes := foundry.DatabaseSize(w.Dir)
		size.Files += files
		size.Bytes += bytes

		out.Worlds = append(out.Worlds, InventoryWorld{
			ID:              w.ID,
			Title:           titleOr(w),
			Description:     w.Description,
			Path:            "worlds/" + w.ID,
			System:          w.System,
			SystemTitle:     system.Title,
			SystemVersion:   w.SystemVersion,
			SystemInstalled: hasSystem,
			CoreVersion:     w.CoreVersion,
			LastPlayed:      w.LastPlayed,
			Size:            size,
			ActiveModules:   active,
			MissingModules:  missing,
		})
	}

	modulesPerSystem := map[string]int{}
	for _, m := range inv.Modules {
		for _, id := range m.TargetSystems {
			modulesPerSystem[id]++
		}
	}

	for _, s := range inv.Systems {
		p := describe(s, "systems/"+s.ID, sizes, installed)
		p.UsedByWorlds = worldsBySystem[s.ID]
		p.ModuleCount = modulesPerSystem[s.ID]
		out.Systems = append(out.Systems, p)
	}
	for _, m := range inv.Modules {
		p := describe(m, "modules/"+m.ID, sizes, installed)
		p.UsedByWorlds = enabledIn[m.ID]
		out.Modules = append(out.Modules, p)
	}

	sortByTitle(out.Systems)
	sortByTitle(out.Modules)
	return out
}

func describe(p foundry.Package, path string, sizes map[string]scan.Bucket, installed map[string]bool) InventoryPackage {
	var missing []string
	for _, id := range p.Requires {
		if !installed[id] {
			missing = append(missing, id)
		}
	}

	title := p.Title
	if title == "" {
		title = p.ID
	}
	return InventoryPackage{
		ID:            p.ID,
		Title:         title,
		Version:       p.Version,
		Path:          path,
		Authors:       p.Authors,
		Delivery:      string(p.Delivery),
		Manifest:      p.Manifest,
		Verified:      p.Compat.Verified,
		Minimum:       p.Compat.Minimum,
		Maximum:       p.Compat.Maximum,
		Size:          sizeOf(sizes, path),
		TargetSystems: p.TargetSystems,
		Requires:      p.Requires,
		Missing:       missing,
	}
}

func titleOr(p foundry.Package) string {
	if p.Title != "" {
		return p.Title
	}
	return p.ID
}

func sizeOf(sizes map[string]scan.Bucket, key string) Size {
	b := sizes[key]
	return Size{Files: b.Files, Bytes: b.Bytes}
}

func sortByTitle(list []InventoryPackage) {
	sort.Slice(list, func(i, j int) bool { return list[i].Title < list[j].Title })
}
