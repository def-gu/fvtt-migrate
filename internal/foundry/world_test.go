package foundry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
)

func writeSettings(t *testing.T, worldDir string, entries map[string]string) {
	t.Helper()
	dir := filepath.Join(worldDir, "data", "settings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for key, value := range entries {
		doc, err := json.Marshal(map[string]any{"key": key, "value": value})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Put([]byte("!settings!"+key), doc, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestActiveModules(t *testing.T) {
	world := t.TempDir()
	writeSettings(t, world, map[string]string{
		"core.compendiumConfiguration": `{"pf2e.bestiary":{"folder":"x"}}`,
		"core.moduleConfiguration":     `{"babele":true,"autoanimations":true,"coc-chest":false}`,
	})

	got, err := ActiveModules(world)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"autoanimations", "babele"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("active = %v, want %v", got, want)
	}
}

func TestActiveModulesWithoutSettings(t *testing.T) {
	got, err := ActiveModules(t.TempDir())
	if err != nil || got != nil {
		t.Errorf("active = %v, %v; want nil, nil", got, err)
	}
}
