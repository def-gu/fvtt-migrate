package foundry

import (
	"os"
	"path/filepath"
	"runtime"
)

func Candidates() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	switch runtime.GOOS {
	case "windows":
		var out []string
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			out = append(out, filepath.Join(local, "FoundryVTT"))
		}
		out = append(out,
			filepath.Join(home, "AppData", "Local", "FoundryVTT"),
			filepath.Join(home, "FoundryVTT"),
		)
		return append(out, drives()...)
	case "darwin":
		return []string{
			filepath.Join(home, "Library", "Application Support", "FoundryVTT"),
			filepath.Join(home, "FoundryVTT"),
		}
	default:
		return []string{
			filepath.Join(home, ".local", "share", "FoundryVTT"),
			filepath.Join(home, "foundrydata"),
			filepath.Join(home, "FoundryVTT"),
			"/srv/foundry-data",
		}
	}
}

// Foundry is commonly installed on the system drive and told to keep its data
// on a larger one, where the folder sits at the root under a handful of names.
func drives() []string {
	names := []string{"FoundryVTT", "FoundryData", "foundrydata", "Foundry"}

	var out []string
	for letter := 'D'; letter <= 'Z'; letter++ {
		root := string(letter) + `:\`
		if _, err := os.Stat(root); err != nil {
			continue
		}
		for _, name := range names {
			out = append(out, filepath.Join(root, name))
		}
	}
	return out
}

func Discover() (*Install, error) {
	found := DiscoverAll()
	if len(found) == 0 {
		return nil, ErrNotFoundry
	}
	return found[0], nil
}

// Every candidate is returned rather than the first, because an installation
// kept on a second drive is the normal case and the panel has to offer a
// choice instead of picking one.
func DiscoverAll() []*Install {
	var out []*Install
	seen := map[string]bool{}
	for _, p := range Candidates() {
		inst, err := Open(p)
		if err != nil || seen[inst.Root] {
			continue
		}
		seen[inst.Root] = true
		out = append(out, inst)
	}
	return out
}
