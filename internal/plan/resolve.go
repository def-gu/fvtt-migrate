package plan

import (
	"sort"
	"strings"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/upstream"
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

	ManifestURL string `yaml:"manifest_url,omitempty"`
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

type Options struct {
	TargetCore   string
	ObservedCore string
	Cache        Cache
	Updates      map[string]upstream.Result
}

func ResolvePackages(inv *foundry.Inventory, opts Options) []Package {
	pkgs := append(append([]foundry.Package{}, inv.Systems...), inv.Modules...)

	out := make([]Package, 0, len(pkgs))
	for _, p := range pkgs {
		out = append(out, resolveOne(p, opts))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func resolveOne(p foundry.Package, opts Options) Package {
	premium := p.DeclaresManifest && p.Manifest == ""
	declared := version.Check(p.Compat.Minimum, p.Compat.Verified, p.Compat.Maximum, opts.TargetCore)
	update, checked := opts.Updates[p.ID]

	r := Package{
		ID:             p.ID,
		Kind:           string(p.Kind),
		Version:        p.Version,
		Policy:         PolicyPin,
		Premium:        premium,
		CompatDeclared: string(declared),
		ObservedOn:     opts.ObservedCore,
		Entitlement:    EntitledBySource,
		Shared:         !premium && p.Manifest != "",
	}

	haveUpdate := checked && update.Available != "" && version.Compare(update.Available, p.Version) > 0
	var availableVerdict version.Verdict
	if haveUpdate {
		c := update.AvailableCompat
		availableVerdict = version.Check(c.Minimum, c.Verified, c.Maximum, opts.TargetCore)
		r.Available = update.Available
		r.CompatAvailable = string(availableVerdict)
		r.Widens = Widens(c.Minimum, c.Verified, opts.TargetCore)
	}
	r.Recommend = Recommend(declared, availableVerdict, haveUpdate)

	source, reason := chooseSource(p, premium, opts, update)
	r.Source, r.Reason = source, reason
	if source == FromManifest {
		r.ManifestURL = pinnedURL(p, update)
	}
	return r
}

func chooseSource(p foundry.Package, premium bool, opts Options, update upstream.Result) (Source, string) {
	switch {
	case premium:
		// Premium packages are entitled to the buyer's account. They are never
		// fetched by the target and never shared between tenants.
		return FromUpload, "premium content"
	case !p.DeclaresManifest:
		return FromUpload, "no manifest, local development"
	}

	if opts.Cache != nil && opts.Cache.Has(string(p.Kind), p.ID, p.Version) {
		return FromCache, ""
	}
	if p.Version == "" {
		return FromUpload, "manifest declares no version"
	}
	if pinnedURL(p, update) != "" {
		return FromManifest, ""
	}
	if update.Available != "" {
		return FromUpload, "upstream now serves " + update.Available + ", not " + p.Version
	}
	if update.Err != nil {
		return FromUpload, "manifest unreachable: " + update.Err.Error()
	}
	return FromUpload, "no source pinned to " + p.Version
}

func pinnedURL(p foundry.Package, update upstream.Result) string {
	if update.PinnedManifest != "" {
		return update.PinnedManifest
	}
	if p.Download != "" && strings.Contains(p.Download, p.Version) {
		return p.Manifest
	}
	if update.Available != "" && update.Available == p.Version {
		return p.Manifest
	}
	return ""
}
