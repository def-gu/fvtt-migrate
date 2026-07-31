package scan

import (
	"sort"
	"strings"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
)

type Class int

const (
	// Referenced files are reachable from a document in some world.
	Referenced Class = iota
	// Packaged files sit inside a module or system directory. They move with
	// their package because modules load assets in ways no static scan sees.
	Packaged
	// Orphan files live in user upload directories with nothing pointing at them.
	Orphan
)

type Bucket struct {
	Files int
	Bytes int64
}

type Missing struct {
	Path  string
	Refs  int
	Where string
}

type Summary struct {
	Referenced Bucket
	Packaged   Bucket
	Orphans    Bucket

	OrphansByDir map[string]Bucket
	BrokenByDir  map[string]int
	Broken       []Missing
	CaseIssues   []Missing
	CoreRefs     int
	UniqueRefs   int
}

// Renamed reports directories holding orphaned files that broken references
// also point into. That combination means paths went stale, not that the files
// are junk: skipping them would migrate a world with broken scenes.
func (s *Summary) Renamed() []string {
	// Broken references under these are missing packages, a different fault.
	namespaces := map[string]bool{"modules": true, "systems": true, "worlds": true}

	var out []string
	for dir := range s.OrphansByDir {
		if !namespaces[dir] && s.BrokenByDir[dir] > 0 {
			out = append(out, dir)
		}
	}
	sort.Slice(out, func(i, j int) bool { return s.BrokenByDir[out[i]] > s.BrokenByDir[out[j]] })
	return out
}

func Analyze(inv *foundry.Inventory, ix *Index) (*Summary, error) {
	s := &Summary{
		OrphansByDir: map[string]Bucket{},
		BrokenByDir:  map[string]int{},
	}

	referenced := map[string]bool{}
	broken := map[string]*Missing{}
	caseIssues := map[string]*Missing{}
	seen := map[string]bool{}

	record := func(r Ref) {
		if !seen[r.Path] {
			seen[r.Path] = true
			switch loc, _ := ix.Lookup(r.Path); loc {
			case InData:
				referenced[r.Path] = true
			case InCore:
				s.CoreRefs++
			case CaseMismatch:
				referenced[strings.ToLower(r.Path)] = true
				caseIssues[r.Path] = &Missing{Path: r.Path, Where: r.Where}
			case NotFound:
				broken[r.Path] = &Missing{Path: r.Path, Where: r.Where}
				s.BrokenByDir[topDir(r.Path)]++
			}
		}
		if m, ok := broken[r.Path]; ok {
			m.Refs++
		}
		if m, ok := caseIssues[r.Path]; ok {
			m.Refs++
		}
	}

	for _, w := range inv.Worlds {
		if w.Background != "" {
			if p, ok := normalize(w.Background); ok {
				record(Ref{Path: p, Where: "world.json:background"})
			}
		}
		err := foundry.EachDocument(w.Dir, func(d foundry.Document) error {
			return FromDocument(d.Data, record)
		})
		if err != nil {
			return nil, err
		}
	}
	s.UniqueRefs = len(seen)

	packageDirs := map[string]bool{}
	for _, p := range append(append([]foundry.Package{}, inv.Modules...), inv.Systems...) {
		packageDirs[string(p.Kind)+"s/"+p.ID] = true
	}

	ix.Each(func(rel string, size int64) {
		switch classify(rel, referenced, packageDirs) {
		case Referenced:
			s.Referenced.add(size)
		case Packaged:
			s.Packaged.add(size)
		case Orphan:
			s.Orphans.add(size)
			dir := topDir(rel)
			b := s.OrphansByDir[dir]
			b.add(size)
			s.OrphansByDir[dir] = b
		}
	})

	s.Broken = sortMissing(broken)
	s.CaseIssues = sortMissing(caseIssues)
	return s, nil
}

func classify(rel string, referenced map[string]bool, packageDirs map[string]bool) Class {
	if referenced[rel] || referenced[strings.ToLower(rel)] {
		return Referenced
	}
	parts := strings.Split(rel, "/")
	if len(parts) >= 2 && packageDirs[parts[0]+"/"+parts[1]] {
		return Packaged
	}
	if len(parts) >= 2 && parts[0] == "worlds" {
		return Packaged
	}
	return Orphan
}

func topDir(rel string) string {
	if i := strings.Index(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return "."
}

func (b *Bucket) add(size int64) {
	b.Files++
	b.Bytes += size
}

func sortMissing(m map[string]*Missing) []Missing {
	out := make([]Missing, 0, len(m))
	for _, v := range m {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Refs != out[j].Refs {
			return out[i].Refs > out[j].Refs
		}
		return out[i].Path < out[j].Path
	})
	return out
}
