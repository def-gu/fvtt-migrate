package report_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/plan"
	"github.com/def-gu/fvtt-migrate/internal/report"
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

func install(t *testing.T, populated bool) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"Data/worlds", "Data/systems", "Data/modules", "Config"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, root, "Config/options.json", `{}`)

	if populated {
		write(t, root, "Data/worlds/bare/world.json",
			`{"id":"bare","title":"Пустой","system":"absent","coreVersion":"13.351"}`)
		write(t, root, "Data/modules/bare-module/module.json", `{"id":"bare-module"}`)
	}
	return root
}

// A nil Go slice encodes as null, which is not a list. The panel reads these
// documents as lists and throws on the first null it is handed, which shows as
// a blank page with no message anywhere. Every document the panel receives is
// checked here, empty and populated, because fixing them one field at a time
// has not held.
func TestNoDocumentSentToThePanelHoldsNull(t *testing.T) {
	for _, populated := range []bool{false, true} {
		root := install(t, populated)

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
		sum, err := scan.Analyze(inv, ix, nil)
		if err != nil {
			t.Fatal(err)
		}

		documents := map[string]any{
			"inventory": report.BuildInventory(inst, inv, ix),
			"scan":      report.BuildScan(inst, inv, ix, sum),
			"plan":      plan.Build(inst, inv, sum, plan.Options{TargetCore: "13.351"}),
			"versions":  report.TargetVersions(inv),
		}

		for name, doc := range documents {
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if strings.Contains(string(raw), "null") {
				t.Errorf("with contents=%v, %s holds a null where the panel expects a list\n%s",
					populated, name, raw)
			}
		}
	}
}
