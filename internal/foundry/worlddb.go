package foundry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

type Document struct {
	Collection string
	Namespace  string
	Key        string
	Data       []byte
}

// fog stores explored-area bitmaps as data URIs and never references a file.
var skipCollections = map[string]bool{"fog": true}

// Collections are separate databases, which is what makes a world readable by
// several workers at once.
func Collections(worldDir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(worldDir, "data"))
	if err != nil {
		return nil, err
	}

	var out []string
	for _, e := range entries {
		if e.IsDir() && !skipCollections[e.Name()] {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// Embedded documents live under keys of the form !scenes.tokens!<id>.<id> and
// are walked too. Data is only valid for the duration of the callback.
func EachDocument(worldDir string, fn func(Document) error) error {
	collections, err := Collections(worldDir)
	if err != nil {
		return err
	}

	for _, c := range collections {
		if err := EachInCollection(worldDir, c, fn); err != nil {
			return err
		}
	}
	return nil
}

func EachInCollection(worldDir, collection string, fn func(Document) error) error {
	return eachInCollection(filepath.Join(worldDir, "data", collection), collection, fn)
}

func eachInCollection(dir, collection string, fn func(Document) error) error {
	db, err := leveldb.OpenFile(dir, &opt.Options{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("open %s: %w", collection, err)
	}
	defer db.Close()

	it := db.NewIterator(nil, nil)
	defer it.Release()

	for it.Next() {
		namespace, key := splitKey(string(it.Key()))
		doc := Document{
			Collection: collection,
			Namespace:  namespace,
			Key:        key,
			Data:       it.Value(),
		}
		if err := fn(doc); err != nil {
			return err
		}
	}
	return it.Error()
}

func splitKey(raw string) (namespace, key string) {
	if !strings.HasPrefix(raw, "!") {
		return "", raw
	}
	rest := raw[1:]
	i := strings.Index(rest, "!")
	if i < 0 {
		return "", raw
	}
	return rest[:i], rest[i+1:]
}

// The world database is excluded from the asset index, so its size has to be
// measured separately or a world appears to weigh almost nothing.
func DatabaseSize(worldDir string) (files int, bytes int64) {
	filepath.WalkDir(filepath.Join(worldDir, "data"), func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			files++
			bytes += info.Size()
		}
		return nil
	})
	return files, bytes
}
