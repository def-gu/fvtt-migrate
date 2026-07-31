package plan

import (
	"sort"
	"strings"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/version"
)

type Source string

const (
	// FromCache means the target already holds this exact package version.
	FromCache Source = "server-cache"
	// FromManifest means the target can fetch it itself, at the pinned version.
	FromManifest Source = "manifest"
	// FromUpload means the bytes have to travel from this machine.
	FromUpload Source = "upload"
)

type Policy string

const (
	Pin     Policy = "pin"
	Upgrade Policy = "upgrade"
	Latest  Policy = "latest"
)

type Package struct {
	ID      string `yaml:"id"`
	Kind    string `yaml:"kind"`
	Version string `yaml:"version"`
	Source  Source `yaml:"source"`
	Reason  string `yaml:"reason,omitempty"`
	Compat  string `yaml:"compat"`
	Policy  Policy `yaml:"policy"`
	Premium bool   `yaml:"premium,omitempty"`
	Shared  bool   `yaml:"-"`
}

// Cache answers whether a target already holds a package version. A nil Cache
// resolves offline, which downgrades cache hits to their next best source.
type Cache interface {
	Has(kind, id, versionString string) bool
}

func ResolvePackages(inv *foundry.Inventory, coreVersion string, cache Cache) []Package {
	pkgs := append(append([]foundry.Package{}, inv.Systems...), inv.Modules...)

	out := make([]Package, 0, len(pkgs))
	for _, p := range pkgs {
		out = append(out, resolveOne(p, coreVersion, cache))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func resolveOne(p foundry.Package, coreVersion string, cache Cache) Package {
	premium := p.DeclaresManifest && p.Manifest == ""

	r := Package{
		ID:      p.ID,
		Kind:    string(p.Kind),
		Version: p.Version,
		Policy:  Pin,
		Premium: premium,
		Compat:  string(version.Check(p.Compat.Minimum, p.Compat.Verified, p.Compat.Maximum, coreVersion)),
		Shared:  !premium && p.Manifest != "",
	}

	source, reason := chooseSource(p, premium, cache)
	r.Source, r.Reason = source, reason
	return r
}

func chooseSource(p foundry.Package, premium bool, cache Cache) (Source, string) {
	switch {
	case premium:
		// Premium packages are entitled to the buyer's account. They are never
		// fetched by the target and never shared between tenants.
		return FromUpload, "premium content"
	case !p.DeclaresManifest:
		return FromUpload, "no manifest, local development"
	}

	if cache != nil && cache.Has(string(p.Kind), p.ID, p.Version) {
		return FromCache, ""
	}
	if p.Version == "" {
		return FromUpload, "manifest declares no version"
	}
	// The manifest URL almost always points at "latest", so only a download URL
	// carrying the installed version proves the exact build is still fetchable.
	if p.Download != "" && strings.Contains(p.Download, p.Version) {
		return FromManifest, ""
	}
	return FromUpload, "no download URL pinned to " + p.Version
}
