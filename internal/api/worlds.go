package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
)

func (s *Server) worlds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "method", "GET expected")
		return
	}

	out := WorldsResponse{Worlds: []WorldCount{}}
	entries, err := os.ReadDir(filepath.Join(s.Root, "worlds"))
	if err != nil {
		writeJSON(w, out)
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c := WorldCount{ID: e.Name()}
		err := foundry.EachDocument(filepath.Join(s.Root, "worlds", e.Name()), func(foundry.Document) error {
			c.Documents++
			return nil
		})
		if err != nil {
			c.Failure = err.Error()
		}
		out.Worlds = append(out.Worlds, c)
	}
	sort.Slice(out.Worlds, func(i, j int) bool { return out.Worlds[i].ID < out.Worlds[j].ID })
	writeJSON(w, out)
}
