package content

import (
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/def-gu/fvtt-migrate/internal/progress"
	"lukechampine.com/blake3"
)

type Digest string

type Entry struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime"`
	Digest  Digest `json:"digest"`
}

type Result struct {
	Entries map[string]Entry
	Errors  map[string]error
	Hashed  int
	Reused  int
}

func HashFile(path string) (Digest, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := blake3.New(32, nil)
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return Digest(hex.EncodeToString(h.Sum(nil))), nil
}

func HashTree(root string, rels []string, cache *Cache, workers int) *Result {
	return HashTreeWithProgress(root, rels, cache, workers, nil)
}

func HashTreeWithProgress(root string, rels []string, cache *Cache, workers int, sink progress.Sink) *Result {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	res := &Result{Entries: make(map[string]Entry, len(rels)), Errors: map[string]error{}}
	var mu sync.Mutex

	jobs := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rel := range jobs {
				abs := filepath.Join(root, filepath.FromSlash(rel))
				info, err := os.Stat(abs)
				if err != nil {
					mu.Lock()
					res.Errors[rel] = err
					mu.Unlock()
					continue
				}

				if hit, ok := cache.Lookup(rel, info.Size(), info.ModTime().UnixNano()); ok {
					mu.Lock()
					res.Entries[rel] = hit
					res.Reused++
					mu.Unlock()
					report(sink, res, &mu, len(rels))
					continue
				}

				d, err := HashFile(abs)
				if err != nil {
					mu.Lock()
					res.Errors[rel] = err
					mu.Unlock()
					continue
				}
				e := Entry{Path: rel, Size: info.Size(), ModTime: info.ModTime().UnixNano(), Digest: d}

				mu.Lock()
				res.Entries[rel] = e
				res.Hashed++
				mu.Unlock()
				cache.Store(e)
				report(sink, res, &mu, len(rels))
			}
		}()
	}

	for _, rel := range rels {
		jobs <- rel
	}
	close(jobs)
	wg.Wait()
	return res
}

func report(sink progress.Sink, res *Result, mu *sync.Mutex, total int) {
	if sink == nil {
		return
	}
	mu.Lock()
	done := int64(res.Hashed + res.Reused)
	mu.Unlock()
	progress.Emit(sink, progress.Event{Phase: progress.PhaseHashing, Done: done, Total: int64(total)})
}
