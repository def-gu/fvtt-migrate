package report

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/scan"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildInventoryLinksPackages(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Config/options.json", `{}`)
	write(t, root, "Data/systems/pf2e/system.json", `{"id":"pf2e","title":"Pathfinder Second Edition","version":"7.12.2"}`)
	write(t, root, "Data/worlds/azlant/world.json", `{"id":"azlant","title":"Пути Азланти","system":"pf2e","systemVersion":"7.7.4","coreVersion":"13.351"}`)
	write(t, root, "Data/worlds/orphaned/world.json", `{"id":"orphaned","title":"Без системы","system":"cyberpunk2020","coreVersion":"13.351"}`)
	write(t, root, "Data/modules/dorako-ui/module.json", `{
		"id":"dorako-ui","title":"Dorako UI","version":"3.9.0",
		"authors":[{"name":"Dorako"}],
		"download":"https://example.invalid/dorako.zip",
		"relationships":{"systems":[{"id":"pf2e"}],"requires":[{"id":"absent-helper"}]}}`)
	write(t, root, "Data/modules/dorako-ui/assets/logo.webp", "0123456789")

	inst, err := foundry.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := inst.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	ix, err := scan.Build(inst.Data, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	v := BuildInventory(inst, inv, ix)

	byID := map[string]InventoryWorld{}
	for _, w := range v.Worlds {
		byID[w.ID] = w
	}
	if w := byID["azlant"]; w.Title != "Пути Азланти" || !w.SystemInstalled {
		t.Errorf("azlant = %+v", w)
	}
	if w := byID["orphaned"]; w.SystemInstalled {
		t.Errorf("a world naming an absent system must not report it installed: %+v", w)
	}

	if len(v.Systems) != 1 {
		t.Fatalf("systems = %d, want 1", len(v.Systems))
	}
	s := v.Systems[0]
	if len(s.UsedByWorlds) != 1 || s.UsedByWorlds[0] != "azlant" {
		t.Errorf("system used by %v, want [azlant]", s.UsedByWorlds)
	}
	if s.ModuleCount != 1 {
		t.Errorf("system module count = %d, want 1", s.ModuleCount)
	}

	m := v.Modules[0]
	if m.Delivery != string(foundry.DeliveryOpen) {
		t.Errorf("delivery = %q", m.Delivery)
	}
	if len(m.Missing) != 1 || m.Missing[0] != "absent-helper" {
		t.Errorf("missing requirements = %v", m.Missing)
	}
	if m.Size.Files != 2 || m.Size.Bytes < 10 {
		t.Errorf("module size = %+v, want its manifest and its asset", m.Size)
	}
}
