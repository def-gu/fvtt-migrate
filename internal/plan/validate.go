package plan

import "fmt"

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type Finding struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Where    string   `json:"where"`
	Message  string   `json:"message"`
}

// Validate checks a plan a human may have edited. Errors block applying;
// warnings describe choices that are unusual but the user's to make.
func (p *Plan) Validate() []Finding {
	var out []Finding
	add := func(s Severity, code, where, format string, args ...any) {
		out = append(out, Finding{s, code, where, fmt.Sprintf(format, args...)})
	}

	sources := map[Source]bool{FromCache: true, FromManifest: true, FromUpload: true}
	policies := map[Policy]bool{PolicyPin: true, PolicyUpgrade: true, PolicyLatest: true}

	if p.Source.TargetCore == "" {
		add(SeverityError, "target.missing", "source.target_core_version",
			"no target Foundry version, so no compatibility advice can be trusted")
	}

	seenWorld := map[string]bool{}
	for _, w := range p.Worlds {
		where := "worlds." + w.ID
		if w.ID == "" {
			add(SeverityError, "world.id.missing", "worlds", "a world entry has no id")
			continue
		}
		if seenWorld[w.ID] {
			add(SeverityError, "world.duplicate", where, "world %q appears twice", w.ID)
		}
		seenWorld[w.ID] = true

		if w.Include && !w.SystemInstalled {
			add(SeverityWarning, "world.system.missing", where,
				"world %q is included but its system %q is not installed, so it will not open at the destination",
				w.ID, w.System)
		}
	}

	seenPkg := map[string]bool{}
	for _, pkg := range p.Packages {
		where := "packages." + pkg.ID
		key := pkg.Kind + "/" + pkg.ID
		if seenPkg[key] {
			add(SeverityError, "package.duplicate", where, "package %q appears twice", key)
		}
		seenPkg[key] = true

		if !sources[pkg.Source] {
			add(SeverityError, "package.source.unknown", where,
				"unknown source %q for %s", pkg.Source, pkg.ID)
		}
		if !policies[pkg.Policy] {
			add(SeverityError, "package.policy.unknown", where,
				"unknown policy %q for %s", pkg.Policy, pkg.ID)
		}
		if pkg.Premium && pkg.Source != FromUpload {
			add(SeverityError, "package.premium.source", where,
				"paid package %s must be copied from disk, not fetched", pkg.ID)
		}
		if pkg.Policy != PolicyPin && pkg.Available == "" {
			add(SeverityWarning, "package.upgrade.unknown", where,
				"%s is set to %s but no newer version is known; run plan --check-updates first",
				pkg.ID, pkg.Policy)
		}
	}

	// Broken references under these are missing packages, a different fault
	// from a renamed user directory.
	namespaces := map[string]bool{"modules": true, "systems": true, "worlds": true}

	for _, d := range p.Assets.Directories {
		if d.Action != "include" && d.Action != "skip" {
			add(SeverityError, "directory.action.unknown", "assets."+d.Path,
				"action for %q must be include or skip, not %q", d.Path, d.Action)
		}
		if d.Action == "skip" && d.Broken > 0 && !namespaces[d.Path] {
			add(SeverityWarning, "directory.stale.skipped", "assets."+d.Path,
				"skipping %q while %d broken references point into it will migrate broken scenes",
				d.Path, d.Broken)
		}
	}
	return out
}

func HasErrors(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == SeverityError {
			return true
		}
	}
	return false
}
