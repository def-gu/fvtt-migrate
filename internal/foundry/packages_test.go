package foundry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePackage(t *testing.T, root, sub, name, file, body string) {
	t.Helper()
	dir := filepath.Join(root, "Data", sub, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInventory(t *testing.T) {
	root := newInstall(t)

	writePackage(t, root, "worlds", "azlant", "world.json", `{
		"id":"azlant","title":"Пути Азланти","system":"pf2e",
		"systemVersion":"7.7.4","coreVersion":"13.351",
		"background":"modules/pf2e-kingmaker/assets/x.webp"}`)
	writePackage(t, root, "modules", "lib-wrapper", "module.json", `{
		"id":"lib-wrapper","version":"1.13.5.1",
		"compatibility":{"minimum":"0.6.5","verified":"14"},
		"download":"https://example.invalid/lib-wrapper-v1.13.5.1.zip"}`)
	writePackage(t, root, "modules", "ancient", "module.json", `{
		"name":"ancient","version":"0.1.0",
		"minimumCoreVersion":"0.7.9","compatibleCoreVersion":"0.8.9"}`)
	writePackage(t, root, "modules", "broken", "module.json", `{not json`)

	inst, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := inst.Inventory()
	if err != nil {
		t.Fatal(err)
	}

	if len(inv.Worlds) != 1 {
		t.Fatalf("worlds = %d, want 1", len(inv.Worlds))
	}
	w := inv.Worlds[0]
	if w.System != "pf2e" || w.CoreVersion != "13.351" {
		t.Errorf("world = %+v", w)
	}
	if w.Background == "" {
		t.Error("world background reference was dropped")
	}

	if len(inv.Modules) != 2 {
		t.Fatalf("modules = %d, want 2", len(inv.Modules))
	}
	byID := map[string]Package{}
	for _, m := range inv.Modules {
		byID[m.ID] = m
	}
	if got := byID["lib-wrapper"].Compat.Verified; got != "14" {
		t.Errorf("lib-wrapper verified = %q, want 14", got)
	}
	if got := byID["ancient"].Compat; got.Minimum != "0.7.9" || got.Verified != "0.8.9" {
		t.Errorf("legacy compat not mapped: %+v", got)
	}

	if len(inv.Problems) != 1 {
		t.Errorf("problems = %d, want 1 (the malformed manifest)", len(inv.Problems))
	}
}

func TestDelivery(t *testing.T) {
	root := newInstall(t)

	writePackage(t, root, "modules", "lib-wrapper", "module.json", `{
		"id":"lib-wrapper","version":"1.13.5.1",
		"manifest":"https://github.com/ruipin/fvtt-lib-wrapper/releases/latest/download/module.json",
		"download":"https://github.com/ruipin/fvtt-lib-wrapper/releases/download/v1.13.5.1/module.zip",
		"authors":[{"name":"ruipin"}]}`)
	writePackage(t, root, "modules", "quick-doors", "module.json", `{
		"id":"quick-doors","version":"2.1.0","download":"",
		"manifest":"https://r2.foundryvtt.com/packages-public/quick-doors/module.json",
		"authors":[{"name":"theripper93"}]}`)
	writePackage(t, root, "modules", "pf2e-ap197", "module.json", `{
		"id":"pf2e-ap197","version":"1.0.0","manifest":"","download":"",
		"authors":[{"name":"Paizo"}]}`)

	inst, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := inst.Inventory()
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]Delivery{
		"lib-wrapper": DeliveryOpen,
		"quick-doors": DeliveryStore,
		"pf2e-ap197":  DeliveryCarry,
	}
	for _, m := range inv.Modules {
		if got := m.Delivery; got != want[m.ID] {
			t.Errorf("%s delivery = %q, want %q", m.ID, got, want[m.ID])
		}
		if len(m.Authors) != 1 {
			t.Errorf("%s authors = %v, want one name", m.ID, m.Authors)
		}
	}
}

func TestRelationships(t *testing.T) {
	root := newInstall(t)
	writePackage(t, root, "modules", "pf2e-dorako-ui", "module.json", `{
		"id":"pf2e-dorako-ui","version":"3.9.0",
		"relationships":{
			"systems":[{"id":"pf2e"},{"id":"sf2e"}],
			"requires":[{"id":"lib-wrapper"}],
			"recommends":[{"id":"babele"}]}}`)

	inst, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := inst.Inventory()
	if err != nil {
		t.Fatal(err)
	}

	m := inv.Modules[0]
	if got := strings.Join(m.TargetSystems, ","); got != "pf2e,sf2e" {
		t.Errorf("target systems = %q", got)
	}
	if got := strings.Join(m.Requires, ","); got != "lib-wrapper" {
		t.Errorf("requires = %q", got)
	}
	if got := strings.Join(m.Recommends, ","); got != "babele" {
		t.Errorf("recommends = %q", got)
	}
}
