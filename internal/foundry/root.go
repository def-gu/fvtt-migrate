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
	World    string `json:"world"`
}

type Liveness struct {
	ServerRunning bool
	ActiveWorld   string
}

// Foundry holds Config/options.json.lock while the server process is up, and
// names the loaded world in options.json. Only a loaded world has its database
// open, so a running server on its own does not make a copy unsafe.
func (i *Install) Liveness() Liveness {
	var l Liveness
	if info, err := os.Stat(filepath.Join(i.Config, "options.json.lock")); err == nil && info.IsDir() {
		l.ServerRunning = true
	}
	if opts, err := i.Options(); err == nil {
		l.ActiveWorld = opts.World
	}
	return l
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
