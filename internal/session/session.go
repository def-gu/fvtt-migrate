package session

import (
	"errors"
	"sync"
	"time"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/progress"
	"github.com/def-gu/fvtt-migrate/internal/report"
	"github.com/def-gu/fvtt-migrate/internal/scan"
)

var (
	ErrNoInstallation = errors.New("no installation has been chosen yet")
	ErrNotScanned     = errors.New("the installation has not been read yet")
	ErrBusy           = errors.New("a scan is already running")
)

// Snapshot is one reading of an installation. The panel answers from it instead
// of walking the disk again, so sorting a list or changing a target version
// costs nothing.
type Snapshot struct {
	Install   *foundry.Install
	Inv       *foundry.Inventory
	Index     *scan.Index
	Summary   *scan.Summary
	Inventory *report.Inventory
	Scan      *report.Scan
	At        time.Time
}

type Session struct {
	core string

	mu   sync.RWMutex
	root string
	last *Snapshot

	scan sync.Mutex
}

func New(core string) *Session { return &Session{core: core} }

func (s *Session) Root() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root
}

// Open validates a path and forgets the previous reading, because a snapshot of
// one installation says nothing about another.
func (s *Session) Open(root string) (*foundry.Install, error) {
	inst, err := foundry.Open(root)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root != inst.Root {
		s.last = nil
	}
	s.root = inst.Root
	return inst, nil
}

func (s *Session) Snapshot() (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	switch {
	case s.root == "":
		return nil, ErrNoInstallation
	case s.last == nil:
		return nil, ErrNotScanned
	}
	return s.last, nil
}

func (s *Session) Read(sink progress.Sink) (*Snapshot, error) {
	root := s.Root()
	if root == "" {
		return nil, ErrNoInstallation
	}
	if !s.scan.TryLock() {
		return nil, ErrBusy
	}
	defer s.scan.Unlock()

	snap, err := read(root, s.core, sink)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == root {
		s.last = snap
	}
	return snap, nil
}

func read(root, core string, sink progress.Sink) (*Snapshot, error) {
	inst, err := foundry.Open(root)
	if err != nil {
		return nil, err
	}
	inv, err := inst.Inventory()
	if err != nil {
		return nil, err
	}
	count := int64(len(inv.Worlds) + len(inv.Systems) + len(inv.Modules))
	progress.Emit(sink, progress.Event{Phase: progress.PhasePackages, Done: count, Total: count})

	coreDir := ""
	if core != "" {
		coreDir = core + "/public"
	}
	ix, err := scan.Build(inst.Data, coreDir, sink)
	if err != nil {
		return nil, err
	}
	sum, err := scan.Analyze(inv, ix, sink)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		Install:   inst,
		Inv:       inv,
		Index:     ix,
		Summary:   sum,
		Inventory: report.BuildInventory(inst, inv, ix),
		Scan:      report.BuildScan(inst, inv, ix, sum),
		At:        time.Now(),
	}, nil
}
