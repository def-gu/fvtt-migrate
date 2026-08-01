package verify

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/def-gu/fvtt-migrate/internal/content"
	"github.com/def-gu/fvtt-migrate/internal/foundry"
)

type WorldCheck struct {
	ID         string
	SourceDocs int
	TargetDocs int
	Namespaces []string
	Err        error
}

func (w WorldCheck) OK() bool {
	return w.Err == nil && w.SourceDocs == w.TargetDocs && len(w.Namespaces) == 0
}

type Result struct {
	Missing  []string
	Mismatch []string
	Worlds   []WorldCheck
	Rehashed int
}

func (r *Result) OK() bool {
	if len(r.Missing) > 0 || len(r.Mismatch) > 0 {
		return false
	}
	for _, w := range r.Worlds {
		if !w.OK() {
			return false
		}
	}
	return true
}

// Files reports paths absent from the target, and with deep set, paths whose
// bytes no longer hash to what was sent.
func Files(targetData string, expected map[string]content.Entry, deep bool) (*Result, error) {
	res := &Result{}

	for rel, e := range expected {
		abs := filepath.Join(targetData, filepath.FromSlash(rel))
		info, err := os.Stat(abs)
		if err != nil {
			res.Missing = append(res.Missing, rel)
			continue
		}
		if info.Size() != e.Size {
			res.Mismatch = append(res.Mismatch, rel)
			continue
		}
		if !deep {
			continue
		}

		got, err := content.HashFile(abs)
		res.Rehashed++
		if err != nil || got != e.Digest {
			res.Mismatch = append(res.Mismatch, rel)
		}
	}

	sort.Strings(res.Missing)
	sort.Strings(res.Mismatch)
	return res, nil
}

// Worlds opens every migrated world on both sides and compares what Foundry
// would actually read. A transfer can place every file and still produce a
// world with no contents, which only this check catches.
func Worlds(sourceData, targetData string) ([]WorldCheck, error) {
	entries, err := os.ReadDir(filepath.Join(targetData, "worlds"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []WorldCheck
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		out = append(out, compareWorld(
			filepath.Join(sourceData, "worlds", e.Name()),
			filepath.Join(targetData, "worlds", e.Name()),
			e.Name(),
		))
	}
	return out, nil
}

func compareWorld(sourceDir, targetDir, id string) WorldCheck {
	check := WorldCheck{ID: id}

	source, err := countByNamespace(sourceDir)
	if err != nil {
		check.Err = err
		return check
	}
	target, err := countByNamespace(targetDir)
	if err != nil {
		check.Err = err
		return check
	}

	for ns, n := range source {
		check.SourceDocs += n
		if target[ns] != n {
			check.Namespaces = append(check.Namespaces, ns)
		}
	}
	for ns, n := range target {
		check.TargetDocs += n
		if _, ok := source[ns]; !ok {
			check.Namespaces = append(check.Namespaces, ns)
		}
	}
	sort.Strings(check.Namespaces)
	return check
}

func countByNamespace(worldDir string) (map[string]int, error) {
	counts := map[string]int{}
	err := foundry.EachDocument(worldDir, func(d foundry.Document) error {
		counts[d.Namespace]++
		return nil
	})
	return counts, err
}
