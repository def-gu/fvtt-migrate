package scan

import (
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
)

func TestRealWorldRefs(t *testing.T) {
	root := os.Getenv("FVTT_ROOT")
	if root == "" {
		t.Skip("FVTT_ROOT not set")
	}

	inst, err := foundry.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := inst.Inventory()
	if err != nil {
		t.Fatal(err)
	}

	unique := map[string]bool{}
	roots := map[string]int{}
	byField := map[string]int{}
	var total, parseFail, docs int

	start := time.Now()
	for _, w := range inv.Worlds {
		err := foundry.EachDocument(w.Dir, func(d foundry.Document) error {
			docs++
			if err := FromDocument(d.Data, func(r Ref) {
				total++
				unique[r.Path] = true
				roots[strings.SplitN(r.Path, "/", 2)[0]]++
				byField[r.Where]++
			}); err != nil {
				parseFail++
			}
			return nil
		})
		if err != nil {
			t.Errorf("%s: %v", w.ID, err)
		}
	}

	t.Logf("docs=%d elapsed=%s refs=%d unique=%d parseFail=%d",
		docs, time.Since(start).Round(time.Millisecond), total, len(unique), parseFail)

	t.Logf("--- top-level roots ---")
	for _, kv := range topN(roots, 12) {
		t.Logf("  %-40s %d", kv.k, kv.v)
	}
	t.Logf("--- top fields ---")
	for _, kv := range topN(byField, 12) {
		t.Logf("  %-40s %d", kv.k, kv.v)
	}
}

type kv struct {
	k string
	v int
}

func topN(m map[string]int, n int) []kv {
	out := make([]kv, 0, len(m))
	for k, v := range m {
		out = append(out, kv{k, v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].v > out[j].v })
	if len(out) > n {
		out = out[:n]
	}
	return out
}
