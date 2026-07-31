package plan

import (
	"sort"
	"strings"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/version"
)

type Source string

const (
	FromCache    Source = "server-cache"
	FromManifest Source = "manifest"
	FromUpload   Source = "upload"
)

type Policy string

const (
	PolicyPin     Policy = "pin"
	PolicyUpgrade Policy = "upgrade"
	PolicyLatest  Policy = "latest"
)

type Package struct {
	ID      string `yaml:"id"`
	Kind    string `yaml:"kind"`
	Version string `yaml:"version"`
	Source  Source `yaml:"source"`
	Reason  string `yaml:"reason,omitempty"`
	Policy  Policy `yaml:"policy"`
	Premium bool   `yaml:"premium,omitempty"`

	CompatDeclared  string         `yaml:"compat_declared"`
	ObservedOn      string         `yaml:"observed_on,omitempty"`
	Available       string         `yaml:"available,omitempty"`
	CompatAvailable string         `yaml:"compat_available,omitempty"`
	Recommend       Recommendation `yaml:"recommend"`
	Widens          bool           `yaml:"widens_support,omitempty"`

	Entitlement string `yaml:"entitled_by"`

	// An optimisation, never an authorisation: entitlement is checked first and
	// independently of where the bytes come from.
	Shared bool `yaml:"-"`
}

// Having had the package is the only basis on which it is ever handed back.
const EntitledBySource = "present-in-source-install"

// A nil Cache resolves offline: hits downgrade to their next best source.
type Cache interface {
	Has(kind, id, versionString string) bool
}

func ResolvePackages(inv *foundry.Inventory, targetCore, observedCore string, cache Cache) []Package {
	pkgs := append(append([]foundry.Package{}, inv.Systems...), inv.Modules...)

	out := make([]Package, 0, len(pkgs))
	for _, p := range pkgs {
		out = append(out, resolveOne(p, targetCore, observedCore, cache))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func resolveOne(p foundry.Package, targetCore, observedCore string, cache Cache) Package {
	premium := p.DeclaresManifest && p.Manifest == ""
	declared := version.Check(p.Compat.Minimum, p.Compat.Verified, p.Compat.Maximum, targetCore)

	r := Package{
		ID:             p.ID,
		Kind:           string(p.Kind),
		Version:        p.Version,
		Policy:         PolicyPin,
		Premium:        premium,
		CompatDeclared: string(declared),
		ObservedOn:     observedCore,
		Recommend:      Recommend(declared, "", false),
		Entitlement:    EntitledBySource,
		Shared:         !premium && p.Manifest != "",
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
