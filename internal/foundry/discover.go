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
		return append(out,
			filepath.Join(home, "AppData", "Local", "FoundryVTT"),
			filepath.Join(home, "FoundryVTT"),
		)
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

func Discover() (*Install, error) {
	for _, p := range Candidates() {
		if inst, err := Open(p); err == nil {
			return inst, nil
		}
	}
	return nil, ErrNotFoundry
}
