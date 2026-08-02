package foundry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const moduleConfigurationKey = "core.moduleConfiguration"

// A world names the modules it enables in its own settings, so what a world
// needs is a fact recorded by the game master rather than a guess from
// manifests.
func ActiveModules(worldDir string) ([]string, error) {
	dir := filepath.Join(worldDir, "data", "settings")
	if _, err := os.Stat(dir); err != nil {
		return nil, nil
	}

	db, err := leveldb.OpenFile(dir, &opt.Options{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer db.Close()

	it := db.NewIterator(nil, nil)
	defer it.Release()

	for it.Next() {
		var s struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(it.Value(), &s); err != nil || s.Key != moduleConfigurationKey {
			continue
		}
		return enabledFrom(s.Value)
	}
	return nil, it.Error()
}

// The value is stored as a JSON string holding JSON.
func enabledFrom(value json.RawMessage) ([]string, error) {
	raw := []byte(value)
	var inner string
	if json.Unmarshal(raw, &inner) == nil {
		raw = []byte(inner)
	}

	var config map[string]bool
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(config))
	for id, on := range config {
		if on {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out, nil
}
