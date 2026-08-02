package api

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
)

type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Foundry bool   `json:"foundry"`
}

type Listing struct {
	Path    string   `json:"path"`
	Parent  string   `json:"parent"`
	Roots   []string `json:"roots"`
	Entries []Entry  `json:"entries"`
}

type Found struct {
	Root    string `json:"root"`
	Worlds  int    `json:"worlds"`
	Systems int    `json:"systems"`
	Modules int    `json:"modules"`
}

// Detect reads only the manifests, not the assets, so the welcome screen can
// name what it found before anything long has started.
func Detect() []Found {
	out := []Found{}
	for _, inst := range foundry.DiscoverAll() {
		f := Found{Root: inst.Root}
		if inv, err := inst.Inventory(); err == nil {
			f.Worlds, f.Systems, f.Modules = len(inv.Worlds), len(inv.Systems), len(inv.Modules)
		}
		out = append(out, f)
	}
	return out
}

func Browse(path string) (*Listing, error) {
	if path == "" {
		path = defaultDir()
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	list := &Listing{Path: abs, Roots: browseRoots(), Entries: []Entry{}}
	if parent := filepath.Dir(abs); parent != abs {
		list.Parent = parent
	}

	for _, e := range entries {
		if !e.IsDir() || e.Name()[0] == '.' {
			continue
		}
		child := filepath.Join(abs, e.Name())
		_, err := foundry.Open(child)
		list.Entries = append(list.Entries, Entry{Name: e.Name(), Path: child, Foundry: err == nil})
	}
	sort.Slice(list.Entries, func(i, j int) bool { return list.Entries[i].Name < list.Entries[j].Name })
	return list, nil
}

func defaultDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return string(filepath.Separator)
}

func browseRoots() []string {
	out := []string{}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, home)
	}
	if runtime.GOOS != "windows" {
		return append(out, string(filepath.Separator))
	}
	for letter := 'A'; letter <= 'Z'; letter++ {
		root := string(letter) + `:\`
		if _, err := os.Stat(root); err == nil {
			out = append(out, root)
		}
	}
	return out
}
