package foundry

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Kind string

const (
	KindWorld  Kind = "world"
	KindSystem Kind = "system"
	KindModule Kind = "module"
)

type Compatibility struct {
	Minimum  string `json:"minimum,omitempty"`
	Verified string `json:"verified,omitempty"`
	Maximum  string `json:"maximum,omitempty"`
}

// Delivery is read from the download field rather than from `protected`, which
// Foundry rewrites to false on install.
type Delivery string

const (
	DeliveryOpen  Delivery = "open"
	DeliveryStore Delivery = "store"
	DeliveryCarry Delivery = "carry"
)

var storeHosts = map[string]bool{
	"r2.foundryvtt.com": true,
	"foundryvtt.com":    true,
	"cdn.paizo.com":     true,
}

type Package struct {
	Kind     Kind
	ID       string
	Title    string
	Version  string
	Dir      string
	Manifest string
	Download string
	Authors  []string
	Delivery Delivery
	Compat   Compatibility

	TargetSystems []string
	Requires      []string
	Recommends    []string

	// DeclaresManifest distinguishes a manifest field left deliberately empty,
	// which is how Foundry marks premium content delivered through the user's
	// account, from one that was never written, which means local development.
	DeclaresManifest bool

	System        string
	SystemVersion string
	CoreVersion   string
	Background    string
	Description   string
	LastPlayed    string
}

type Problem struct {
	Dir    string
	Reason string
}

type Inventory struct {
	Worlds   []Package
	Systems  []Package
	Modules  []Package
	Problems []Problem
}

// laxString accepts a JSON number where the manifest schema says string.
// Roughly one module in twelve writes `"minimum": 13` rather than `"13"`.
type laxString string

func (s *laxString) UnmarshalJSON(b []byte) error {
	text := string(b)
	if text == "null" {
		*s = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*s = laxString(v)
		return nil
	}
	*s = laxString(text)
	return nil
}

type rawCompat struct {
	Minimum  laxString `json:"minimum"`
	Verified laxString `json:"verified"`
	Maximum  laxString `json:"maximum"`
}

type rawManifest struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Title    string    `json:"title"`
	Version  laxString `json:"version"`
	Manifest *string   `json:"manifest"`
	Download string    `json:"download"`
	Author   string    `json:"author"`
	Authors  []struct {
		Name string `json:"name"`
	} `json:"authors"`

	Compatibility         *rawCompat `json:"compatibility"`
	MinimumCoreVersion    laxString  `json:"minimumCoreVersion"`
	CompatibleCoreVersion laxString  `json:"compatibleCoreVersion"`

	System        string    `json:"system"`
	SystemVersion laxString `json:"systemVersion"`
	CoreVersion   laxString `json:"coreVersion"`
	Background    string    `json:"background"`
	Description   string    `json:"description"`
	LastPlayed    string    `json:"lastPlayed"`

	Relationships *struct {
		Systems    []related `json:"systems"`
		Requires   []related `json:"requires"`
		Recommends []related `json:"recommends"`
	} `json:"relationships"`
}

type related struct {
	ID string `json:"id"`
}

func ids(list []related) []string {
	var out []string
	for _, r := range list {
		if r.ID != "" {
			out = append(out, r.ID)
		}
	}
	return out
}

var manifestFile = map[Kind]string{
	KindWorld:  "world.json",
	KindSystem: "system.json",
	KindModule: "module.json",
}

var subdir = map[Kind]string{
	KindWorld:  "worlds",
	KindSystem: "systems",
	KindModule: "modules",
}

func (i *Install) Inventory() (*Inventory, error) {
	inv := &Inventory{}
	for _, kind := range []Kind{KindWorld, KindSystem, KindModule} {
		pkgs, problems, err := i.scanKind(kind)
		if err != nil {
			return nil, err
		}
		switch kind {
		case KindWorld:
			inv.Worlds = pkgs
		case KindSystem:
			inv.Systems = pkgs
		case KindModule:
			inv.Modules = pkgs
		}
		inv.Problems = append(inv.Problems, problems...)
	}
	return inv, nil
}

func (i *Install) scanKind(kind Kind) ([]Package, []Problem, error) {
	base := filepath.Join(i.Data, subdir[kind])
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var pkgs []Package
	var problems []Problem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(base, e.Name())
		pkg, err := readPackage(kind, dir)
		if err != nil {
			problems = append(problems, Problem{Dir: dir, Reason: err.Error()})
			continue
		}
		pkgs = append(pkgs, *pkg)
	}
	return pkgs, problems, nil
}

func readPackage(kind Kind, dir string) (*Package, error) {
	raw, err := os.ReadFile(filepath.Join(dir, manifestFile[kind]))
	if err != nil {
		return nil, err
	}
	p, err := ParseManifest(kind, raw)
	if err != nil {
		return nil, err
	}
	p.Dir = dir
	if p.ID == "" {
		p.ID = filepath.Base(dir)
	}
	return p, nil
}

func ParseManifest(kind Kind, raw []byte) (*Package, error) {
	var m rawManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestFile[kind], err)
	}

	id := m.ID
	if id == "" {
		id = m.Name
	}

	manifest := ""
	if m.Manifest != nil {
		manifest = strings.TrimSpace(*m.Manifest)
	}

	download := strings.TrimSpace(m.Download)

	var targetSystems, requires, recommends []string
	if r := m.Relationships; r != nil {
		targetSystems, requires, recommends = ids(r.Systems), ids(r.Requires), ids(r.Recommends)
	}

	return &Package{
		Kind:             kind,
		ID:               id,
		Title:            m.Title,
		Version:          string(m.Version),
		Manifest:         manifest,
		DeclaresManifest: m.Manifest != nil,
		Download:         download,
		Authors:          m.authors(),
		Delivery:         delivery(manifest, download),
		Compat:           m.compat(),
		TargetSystems:    targetSystems,
		Requires:         requires,
		Recommends:       recommends,
		System:           m.System,
		SystemVersion:    string(m.SystemVersion),
		CoreVersion:      string(m.CoreVersion),
		Background:       m.Background,
		Description:      m.Description,
		LastPlayed:       m.LastPlayed,
	}, nil
}

func (m *rawManifest) authors() []string {
	var out []string
	for _, a := range m.Authors {
		if name := strings.TrimSpace(a.Name); name != "" {
			out = append(out, name)
		}
	}
	if len(out) == 0 && strings.TrimSpace(m.Author) != "" {
		out = append(out, strings.TrimSpace(m.Author))
	}
	return out
}

func delivery(manifest, download string) Delivery {
	if download != "" {
		return DeliveryOpen
	}
	if u, err := url.Parse(manifest); err == nil && storeHosts[u.Hostname()] {
		return DeliveryStore
	}
	return DeliveryCarry
}

// Manifests written before Foundry v10 carry the bounds as two flat fields.
func (m *rawManifest) compat() Compatibility {
	if m.Compatibility != nil {
		return Compatibility{
			Minimum:  string(m.Compatibility.Minimum),
			Verified: string(m.Compatibility.Verified),
			Maximum:  string(m.Compatibility.Maximum),
		}
	}
	return Compatibility{
		Minimum:  string(m.MinimumCoreVersion),
		Verified: string(m.CompatibleCoreVersion),
	}
}
