package plan

import "testing"

func codes(f []Finding) map[string]Finding {
	out := map[string]Finding{}
	for _, x := range f {
		out[x.Code] = x
	}
	return out
}

func validPlan() *Plan {
	return &Plan{
		Format: Format{Version: 1, Capabilities: writtenCapabilities},
		Source: Endpoint{TargetCore: "13.351", PackageMode: PolicyPin},
		Worlds: []World{{ID: "w", System: "pf2e", SystemInstalled: true, Include: true}},
		Packages: []Package{
			{ID: "m", Kind: "module", Source: FromManifest, Policy: PolicyPin},
		},
		Assets: Assets{Directories: []Directory{{Path: "Карты", Action: "include"}}},
	}
}

func TestValidateAcceptsAGeneratedPlan(t *testing.T) {
	if f := validPlan().Validate(); len(f) != 0 {
		t.Errorf("a freshly generated plan was rejected: %+v", f)
	}
}

func TestValidateRejectsUnknownEnums(t *testing.T) {
	p := validPlan()
	p.Packages[0].Source = "carrier-pigeon"
	p.Packages[0].Policy = "whatever"
	p.Assets.Directories[0].Action = "maybe"

	got := codes(p.Validate())
	for _, code := range []string{"package.source.unknown", "package.policy.unknown", "directory.action.unknown"} {
		if _, ok := got[code]; !ok {
			t.Errorf("%s not reported", code)
		}
	}
	if !HasErrors(p.Validate()) {
		t.Error("unknown enums did not block applying")
	}
}

func TestValidateRefusesToFetchPaidContent(t *testing.T) {
	p := validPlan()
	p.Packages[0].Premium = true
	p.Packages[0].Source = FromCache

	if _, ok := codes(p.Validate())["package.premium.source"]; !ok {
		t.Error("a paid package pointed at the shared cache was accepted")
	}
}

func TestValidateWarnsOnChoicesThatAreTheUsersToMake(t *testing.T) {
	p := validPlan()
	p.Worlds[0].SystemInstalled = false
	p.Assets.Directories[0].Action = "skip"
	p.Assets.Directories[0].Broken = 71

	got := codes(p.Validate())
	for _, code := range []string{"world.system.missing", "directory.stale.skipped"} {
		f, ok := got[code]
		if !ok {
			t.Fatalf("%s not reported", code)
		}
		if f.Severity != SeverityWarning {
			t.Errorf("%s is %q; the user is allowed to make this choice", code, f.Severity)
		}
	}
	if HasErrors(p.Validate()) {
		t.Error("warnings blocked applying")
	}
}

func TestValidateCatchesDuplicates(t *testing.T) {
	p := validPlan()
	p.Worlds = append(p.Worlds, p.Worlds[0])
	p.Packages = append(p.Packages, p.Packages[0])

	got := codes(p.Validate())
	if _, ok := got["world.duplicate"]; !ok {
		t.Error("duplicate world not reported")
	}
	if _, ok := got["package.duplicate"]; !ok {
		t.Error("duplicate package not reported")
	}
}
