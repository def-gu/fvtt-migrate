package plan

import (
	"io"
	"runtime"
	"sort"
	"strings"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/scan"
	"github.com/def-gu/fvtt-migrate/internal/version"
	"gopkg.in/yaml.v3"
)

const formatVersion = 1

// Capabilities name the features a document uses, so that additive changes need
// no version bump and a reader can refuse precisely what it does not know.
var writtenCapabilities = []string{
	"plan/1",
	"digest/blake3-256",
	"transfer/whole-file",
}

type Format struct {
	Version      int      `yaml:"version" json:"version"`
	Capabilities []string `yaml:"capabilities" json:"capabilities"`
}

type Identity struct {
	Tenant       string `yaml:"tenant,omitempty" json:"tenant,omitempty"`
	Device       string `yaml:"device,omitempty" json:"device,omitempty"`
	Installation string `yaml:"installation,omitempty" json:"installation,omitempty"`
}

// Baton records which installation last held a world and the state both sides
// agreed on, so a later handoff can tell divergence from a fresh copy.
type Baton struct {
	Holder     string `yaml:"holder" json:"holder"`
	Generation int    `yaml:"generation" json:"generation"`
	BaseDigest string `yaml:"base_digest,omitempty" json:"base_digest,omitempty"`
}

type Plan struct {
	Format   Format    `yaml:"format" json:"format"`
	Identity Identity  `yaml:"identity" json:"identity"`
	Source   Endpoint  `yaml:"source" json:"source"`
	Worlds   []World   `yaml:"worlds" json:"worlds"`
	Packages []Package `yaml:"packages" json:"packages"`
	Assets   Assets    `yaml:"assets" json:"assets"`
}

type UnsupportedError struct {
	Unknown []string
}

func (e *UnsupportedError) Error() string {
	return "this plan needs features this version does not have: " +
		strings.Join(e.Unknown, ", ") + ". Update fvtt-migrate and try again."
}

func (f Format) Check() error {
	known := map[string]bool{}
	for _, c := range writtenCapabilities {
		known[c] = true
	}

	var unknown []string
	for _, c := range f.Capabilities {
		if !known[c] {
			unknown = append(unknown, c)
		}
	}
	if len(unknown) > 0 {
		return &UnsupportedError{Unknown: unknown}
	}
	return nil
}

type Endpoint struct {
	Root        string `yaml:"root" json:"root"`
	OS          string `yaml:"os" json:"os"`
	TargetCore  string `yaml:"target_core_version" json:"target_core_version"`
	PackageMode Policy `yaml:"package_policy" json:"package_policy"`
}

type World struct {
	ID              string `yaml:"id" json:"id"`
	System          string `yaml:"system" json:"system"`
	SystemVersion   string `yaml:"system_version" json:"system_version"`
	CoreVersion     string `yaml:"core_version" json:"core_version"`
	SystemInstalled bool   `yaml:"system_installed" json:"system_installed"`
	Include         bool   `yaml:"include" json:"include"`
	Blocker         string `yaml:"blocker,omitempty" json:"blocker,omitempty"`
	Baton           *Baton `yaml:"baton,omitempty" json:"baton,omitempty"`
}

type Assets struct {
	Referenced  Bucket      `yaml:"referenced" json:"referenced"`
	Packaged    Bucket      `yaml:"in_packages" json:"in_packages"`
	Directories []Directory `yaml:"user_directories" json:"user_directories"`
	BrokenRefs  int         `yaml:"broken_references" json:"broken_references"`
	CaseOnly    int         `yaml:"case_only_matches" json:"case_only_matches"`
}

type Bucket struct {
	Files int   `yaml:"files" json:"files"`
	Bytes int64 `yaml:"bytes" json:"bytes"`
}

type Directory struct {
	Path   string `yaml:"path" json:"path"`
	Files  int    `yaml:"files" json:"files"`
	Bytes  int64  `yaml:"bytes" json:"bytes"`
	Action string `yaml:"action" json:"action"`
	Note   string `yaml:"note,omitempty" json:"note,omitempty"`
	Broken int    `yaml:"broken_references_into_it,omitempty" json:"broken_references_into_it,omitempty"`
}

func Build(inst *foundry.Install, inv *foundry.Inventory, sum *scan.Summary, opts Options) *Plan {
	opts.ObservedCore = highestCore(inv)
	targetCore := opts.TargetCore
	if targetCore == "" {
		targetCore = opts.ObservedCore
		opts.TargetCore = targetCore
	}

	installed := map[string]bool{}
	for _, s := range inv.Systems {
		installed[s.ID] = true
	}

	p := &Plan{
		Format:   Format{Version: formatVersion, Capabilities: writtenCapabilities},
		Identity: opts.Identity,
		Source: Endpoint{
			Root:        inst.Root,
			OS:          runtime.GOOS,
			TargetCore:  targetCore,
			PackageMode: PolicyPin,
		},
		Packages: ResolvePackages(inv, opts),
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
	if err := p.Format.Check(); err != nil {
		return nil, err
	}
	return &p, nil
}
