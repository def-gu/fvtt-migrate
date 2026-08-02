package scan

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/progress"
)

type Class int

const (
	Referenced Class = iota
	// Packaged files move with their package: modules load assets in ways no
	// static scan sees, so their directories are never analysed.
	Packaged
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

	referenced  map[string]bool
	packageDirs map[string]bool
}

func (s *Summary) Classify(rel string) Class {
	return classify(rel, s.referenced, s.packageDirs)
}

// Orphans plus broken references into the same directory mean stale paths,
// not unused files: skipping them would migrate a world with broken scenes.
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

func Analyze(inv *foundry.Inventory, ix *Index, sink progress.Sink) (*Summary, error) {
	s := &Summary{
		OrphansByDir: map[string]Bucket{},
		BrokenByDir:  map[string]int{},
	}

	referenced := map[string]bool{}
	broken := map[string]*Missing{}
	caseIssues := map[string]*Missing{}
	seen := map[string]bool{}

	var mu sync.Mutex
	record := func(r Ref) {
		mu.Lock()
		defer mu.Unlock()
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
	}
	if err := readWorlds(inv, record, sink); err != nil {
		return nil, err
	}
	s.UniqueRefs = len(seen)

	packageDirs := map[string]bool{}
	for _, p := range append(append([]foundry.Package{}, inv.Modules...), inv.Systems...) {
		packageDirs[string(p.Kind)+"s/"+p.ID] = true
	}

	classified := int64(0)
	ix.Each(func(rel string, size int64) {
		classified++
		progress.Emit(sink, progress.Event{
			Phase: progress.PhaseClassifying, Done: classified, Total: int64(ix.Len()),
		})
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

	s.referenced = referenced
	s.packageDirs = packageDirs
	s.Broken = sortMissing(broken)
	s.CaseIssues = sortMissing(caseIssues)
	return s, nil
}

type unit struct {
	world      string
	title      string
	dir        string
	collection string
}

func units(inv *foundry.Inventory) ([]unit, error) {
	var out []unit
	for _, w := range inv.Worlds {
		collections, err := foundry.Collections(w.Dir)
		if err != nil {
			return nil, err
		}
		title := w.Title
		if title == "" {
			title = w.ID
		}
		for _, c := range collections {
			out = append(out, unit{world: w.ID, title: title, dir: w.Dir, collection: c})
		}
	}
	return out, nil
}

func readWorlds(inv *foundry.Inventory, record func(Ref), sink progress.Sink) error {
	work, err := units(inv)
	if err != nil {
		return err
	}

	jobs := make(chan unit)
	errs := make(chan error, 1)
	var wg sync.WaitGroup
	var done atomic.Int64

	for i := 0; i < workers(len(work)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range jobs {
				err := foundry.EachInCollection(u.dir, u.collection, func(d foundry.Document) error {
					return FromDocument(d.Data, record)
				})
				if err != nil {
					select {
					case errs <- fmt.Errorf("%s/%s: %w", u.world, u.collection, err):
					default:
					}
				}
				progress.Emit(sink, progress.Event{
					Phase:  progress.PhaseWorlds,
					Done:   done.Add(1),
					Total:  int64(len(work)),
					Detail: u.title + " / " + u.collection,
				})
			}
		}()
	}

	for _, u := range work {
		jobs <- u
	}
	close(jobs)
	wg.Wait()

	select {
	case err := <-errs:
		return err
	default:
		return nil
	}
}

func workers(jobs int) int {
	n := runtime.NumCPU()
	if n > jobs {
		n = jobs
	}
	if n < 1 {
		n = 1
	}
	return n
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
