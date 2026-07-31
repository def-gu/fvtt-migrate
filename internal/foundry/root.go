package foundry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Install struct {
	Root   string
	Data   string
	Config string
}

type Options struct {
	DataPath string `json:"dataPath"`
	Language string `json:"language"`
	Port     int    `json:"port"`
}

var ErrNotFoundry = errors.New("not a Foundry user-data directory")

// Open accepts either the user-data root or the Data directory inside it.
func Open(path string) (*Install, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	candidates := []string{abs}
	if filepath.Base(abs) == "Data" {
		candidates = append(candidates, filepath.Dir(abs))
	}

	for _, root := range candidates {
		inst := &Install{
			Root:   root,
			Data:   filepath.Join(root, "Data"),
			Config: filepath.Join(root, "Config"),
		}
		if isDir(inst.Data) && isDir(inst.Config) {
			return inst, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrNotFoundry, abs)
}

func (i *Install) Options() (*Options, error) {
	raw, err := os.ReadFile(filepath.Join(i.Config, "options.json"))
	if err != nil {
		return nil, err
	}
	var o Options
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, fmt.Errorf("parse options.json: %w", err)
	}
	return &o, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
