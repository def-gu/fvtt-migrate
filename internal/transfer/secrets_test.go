package transfer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/plan"
	"github.com/def-gu/fvtt-migrate/internal/scan"
)

// The licence key and the admin password hash live in Config, one level above
// the transfer root. This asserts the separation holds rather than trusting it.
func TestSecretsAreUnreachable(t *testing.T) {
	install := t.TempDir()
	data := filepath.Join(install, "Data")
	config := filepath.Join(install, "Config")

	for _, d := range []string{data, config} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{
		"license.json": `{"license":"SECRET-KEY","signature":"..."}`,
		"admin.txt":    "0123456789abcdef",
		"options.json": `{"port":30000}`,
	} {
		if err := os.WriteFile(filepath.Join(config, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(data, "asset.webp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ix, err := scan.Build(data, "")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := scan.Analyze(&foundry.Inventory{}, ix)
	if err != nil {
		t.Fatal(err)
	}

	sel, err := Select(&plan.Plan{
		Assets: plan.Assets{Directories: []plan.Directory{{Path: "Config", Action: "include"}}},
	}, data, ix, sum)
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range sel.Paths {
		if strings.Contains(p, "license") || strings.Contains(p, "admin") || strings.HasPrefix(p, "Config") {
			t.Errorf("%q entered the selection; secrets must never be transferable", p)
		}
	}
}

func TestPlaceCannotReachConfig(t *testing.T) {
	install := t.TempDir()
	data := filepath.Join(install, "Data")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}

	tgt := NewFileTarget(data)
	for _, rel := range []string{"../Config/license.json", "../../Config/admin.txt"} {
		if _, err := safeJoin(tgt.Root, rel); err == nil {
			t.Errorf("%q resolved inside the transfer root", rel)
		}
	}
}
