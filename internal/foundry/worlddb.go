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

// EachDocument walks every document of a world, including embedded ones, which
// live under keys of the form !scenes.tokens!<sceneId>.<tokenId>.
//
// Data is only valid for the duration of the callback.
func EachDocument(worldDir string, fn func(Document) error) error {
	dataDir := filepath.Join(worldDir, "data")
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if !e.IsDir() || skipCollections[e.Name()] {
			continue
		}
		if err := eachInCollection(filepath.Join(dataDir, e.Name()), e.Name(), fn); err != nil {
			return err
		}
	}
	return nil
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
