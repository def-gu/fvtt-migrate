package foundry

import (
	"os"
	"testing"
)

// Run against a real installation with:
//
//	FVTT_ROOT=/path/to/userdata go test ./internal/foundry -run RealInstall -v
func TestRealInstall(t *testing.T) {
	root := os.Getenv("FVTT_ROOT")
	if root == "" {
		t.Skip("FVTT_ROOT not set")
	}

	inst, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := inst.Inventory()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("worlds=%d systems=%d modules=%d problems=%d",
		len(inv.Worlds), len(inv.Systems), len(inv.Modules), len(inv.Problems))

	for _, w := range inv.Worlds {
		t.Logf("world %-34s system=%s@%s core=%s", w.ID, w.System, w.SystemVersion, w.CoreVersion)
	}
	for _, p := range inv.Problems {
		t.Logf("problem %s: %s", p.Dir, p.Reason)
	}

	var noManifest, noVersion int
	for _, m := range inv.Modules {
		if m.Manifest == "" {
			noManifest++
		}
		if m.Version == "" {
			noVersion++
		}
	}
	t.Logf("modules without manifest=%d without version=%d", noManifest, noVersion)
}
