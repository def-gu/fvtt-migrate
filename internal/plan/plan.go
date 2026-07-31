package plan

import (
	"io"
	"runtime"
	"sort"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/scan"
	"github.com/def-gu/fvtt-migrate/internal/version"
	"gopkg.in/yaml.v3"
)

const formatVersion = 1

type Plan struct {
	FormatVersion int       `yaml:"format_version"`
	Source        Endpoint  `yaml:"source"`
	Worlds        []World   `yaml:"worlds"`
	Packages      []Package `yaml:"packages"`
	Assets        Assets    `yaml:"assets"`
}

type Endpoint struct {
	Root        string `yaml:"root"`
	OS          string `yaml:"os"`
	TargetCore  string `yaml:"target_core_version"`
	PackageMode Policy `yaml:"package_policy"`
}

type World struct {
	ID              string `yaml:"id"`
	System          string `yaml:"system"`
	SystemVersion   string `yaml:"system_version"`
	CoreVersion     string `yaml:"core_version"`
	SystemInstalled bool   `yaml:"system_installed"`
	Include         bool   `yaml:"include"`
	Blocker         string `yaml:"blocker,omitempty"`
}

type Assets struct {
	Referenced  Bucket      `yaml:"referenced"`
	Packaged    Bucket      `yaml:"in_packages"`
	Directories []Directory `yaml:"user_directories"`
	BrokenRefs  int         `yaml:"broken_references"`
	CaseOnly    int         `yaml:"case_only_matches"`
}

type Bucket struct {
	Files int   `yaml:"files"`
	Bytes int64 `yaml:"bytes"`
}

type Directory struct {
	Path   string `yaml:"path"`
	Files  int    `yaml:"files"`
	Bytes  int64  `yaml:"bytes"`
	Action string `yaml:"action"`
	Note   string `yaml:"note,omitempty"`
	Broken int    `yaml:"broken_references_into_it,omitempty"`
}

func Build(inst *foundry.Install, inv *foundry.Inventory, sum *scan.Summary, targetCore string, cache Cache) *Plan {
	if targetCore == "" {
		targetCore = highestCore(inv)
	}

	installed := map[string]bool{}
	for _, s := range inv.Systems {
		installed[s.ID] = true
	}

	p := &Plan{
		FormatVersion: formatVersion,
		Source: Endpoint{
			Root:        inst.Root,
			OS:          runtime.GOOS,
			TargetCore:  targetCore,
			PackageMode: PolicyPin,
		},
		Packages: ResolvePackages(inv, targetCore, cache),
		Assets: Assets{
			Referenced: Bucket{sum.Referenced.Files, sum.Referenced.Bytes},
			Packaged:   Bucket{sum.Packaged.Files, sum.Packaged.Bytes},
			BrokenRefs: len(sum.Broken),
			CaseOnly:   len(sum.CaseIssues),
		},
	}

	for _, w := range inv.Worlds {
		world := World{
			ID:              w.ID,
			System:          w.System,
			SystemVersion:   w.SystemVersion,
			CoreVersion:     w.CoreVersion,
			SystemInstalled: installed[w.System],
			Include:         true,
		}
		if !world.SystemInstalled {
			world.Include = false
			world.Blocker = "system " + w.System + " is not installed"
		}
		p.Worlds = append(p.Worlds, world)
	}

	renamed := map[string]bool{}
	for _, d := range sum.Renamed() {
		renamed[d] = true
	}
	for dir, b := range sum.OrphansByDir {
		d := Directory{
			Path:   dir,
			Files:  b.Files,
			Bytes:  b.Bytes,
			Action: "skip",
			Broken: sum.BrokenByDir[dir],
		}
		if renamed[dir] {
			d.Action = "include"
			d.Note = "unreferenced, but broken references point here: paths look stale, not unused"
		}
		p.Assets.Directories = append(p.Assets.Directories, d)
	}
	sort.Slice(p.Assets.Directories, func(i, j int) bool {
		return p.Assets.Directories[i].Bytes > p.Assets.Directories[j].Bytes
	})

	return p
}

func highestCore(inv *foundry.Inventory) string {
	best := ""
	for _, w := range inv.Worlds {
		if best == "" || version.Compare(w.CoreVersion, best) > 0 {
			best = w.CoreVersion
		}
	}
	return best
}

func (p *Plan) Write(w io.Writer) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(p); err != nil {
		return err
	}
	return enc.Close()
}

func Read(r io.Reader) (*Plan, error) {
	var p Plan
	if err := yaml.NewDecoder(r).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}
