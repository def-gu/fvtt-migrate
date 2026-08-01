package content

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Cache struct {
	mu      sync.RWMutex
	entries map[string]Entry
	path    string
	dirty   bool
}

// The cache lives in the OS cache directory rather than beside the data,
// because the tool must never write into the installation it reads.
func OpenCache(installRoot string) *Cache {
	c := &Cache{entries: map[string]Entry{}}

	dir, err := os.UserCacheDir()
	if err != nil {
		return c
	}
	sum := sha256.Sum256([]byte(installRoot))
	c.path = filepath.Join(dir, "fvtt-migrate", hex.EncodeToString(sum[:8])+".json")
	c.entries = load(c.path)
	return c
}

func load(path string) map[string]Entry {
	out := map[string]Entry{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var stored []Entry
	if json.Unmarshal(raw, &stored) != nil {
		return out
	}
	for _, e := range stored {
		out[e.Path] = e
	}
	return out
}

func (c *Cache) Lookup(rel string, size, modTime int64) (Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.entries[rel]
	if !ok || e.Size != size || e.ModTime != modTime {
		return Entry{}, false
	}
	return e, true
}

func (c *Cache) Entries() []Entry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]Entry, 0, len(c.entries))
	for _, e := range c.entries {
		out = append(out, e)
	}
	return out
}

func (c *Cache) Store(e Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[e.Path] = e
	c.dirty = true
}

func (c *Cache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.dirty || c.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}

	out := make([]Entry, 0, len(c.entries))
	for _, e := range c.entries {
		out = append(out, e)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return err
	}

	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}
