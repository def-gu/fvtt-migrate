package content

import (
	"os"
	"path/filepath"
	"sort"
)

// Recheck reports files whose size or modification time no longer match what
// was hashed. A source that changed under us means the copy is not a copy of
// any single moment, which is exactly what a live Foundry database produces.
func Recheck(root string, entries map[string]Entry) []string {
	var changed []string
	for rel, e := range entries {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || info.Size() != e.Size || info.ModTime().UnixNano() != e.ModTime {
			changed = append(changed, rel)
		}
	}
	sort.Strings(changed)
	return changed
}
