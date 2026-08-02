package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/def-gu/fvtt-migrate/internal/plan"
	"github.com/def-gu/fvtt-migrate/internal/progress"
	"github.com/def-gu/fvtt-migrate/internal/report"
	"github.com/def-gu/fvtt-migrate/internal/session"
	"github.com/def-gu/fvtt-migrate/internal/verify"
)

const (
	PathPing      = "/api/ping"
	PathDetect    = "/api/detect"
	PathBrowse    = "/api/browse"
	PathOpen      = "/api/open"
	PathScan      = "/api/scan"
	PathInventory = "/api/inventory"
	PathState     = "/api/state"
	PathPlan      = "/api/plan"
	PathRun       = "/api/run"
	PathVerify    = "/api/verify"
)

type OpenRequest struct {
	Root string `json:"root"`
}

// Chosen tells the welcome screen what it is looking at without doing any of
// the work: the installation, whether it has been read, and whether Foundry is
// holding a world open.
type Chosen struct {
	Root        string `json:"root"`
	Scanned     bool   `json:"scanned"`
	ScannedAt   string `json:"scanned_at,omitempty"`
	Running     bool   `json:"foundry_running"`
	ActiveWorld string `json:"active_world,omitempty"`
}

type PlanRequest struct {
	TargetCore   string `json:"target_core"`
	CheckUpdates bool   `json:"check_updates"`
}

type RunRequest struct {
	To     string     `json:"to"`
	Token  string     `json:"token"`
	DryRun bool       `json:"dry_run"`
	Plan   *plan.Plan `json:"plan"`
}

type VerifyRequest struct {
	To    string `json:"to"`
	Token string `json:"token"`
}

type State struct {
	Root    string       `json:"root"`
	Targets []string     `json:"targets"`
	Scan    *report.Scan `json:"scan"`
}

// Local drives this machine's own installation. It is bound to the loopback
// address by the command that starts it and carries no token, because reaching
// it already means being at this keyboard.
type Local struct {
	Session *session.Session

	State  func() (*State, error)
	Plan   func(PlanRequest) (*plan.Plan, error)
	Run    func(context.Context, RunRequest, progress.Sink) (any, error)
	Verify func(context.Context, VerifyRequest) (*verify.Result, error)

	mu sync.Mutex
}

func (l *Local) Routes(mux *http.ServeMux) {
	mux.HandleFunc(PathPing, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{"agent": "fvtt-migrate"})
	})
	mux.HandleFunc(PathDetect, l.detect)
	mux.HandleFunc(PathBrowse, l.browse)
	mux.HandleFunc(PathOpen, l.open)
	mux.HandleFunc(PathScan, l.scan)
	mux.HandleFunc(PathInventory, l.inventory)
	mux.HandleFunc(PathState, l.state)
	mux.HandleFunc(PathPlan, l.plan)
	mux.HandleFunc(PathRun, l.run)
	mux.HandleFunc(PathVerify, l.verify)
}

func (l *Local) detect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "method", "GET expected")
		return
	}
	writeJSON(w, Detect())
}

func (l *Local) browse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "method", "GET expected")
		return
	}
	list, err := Browse(r.URL.Query().Get("path"))
	if err != nil {
		fail(w, http.StatusBadRequest, "browse.failed", err.Error())
		return
	}
	writeJSON(w, list)
}

func (l *Local) open(w http.ResponseWriter, r *http.Request) {
	var req OpenRequest
	if !readJSON(w, r) || !decode(w, r, &req) {
		return
	}
	if _, err := l.Session.Open(req.Root); err != nil {
		fail(w, http.StatusBadRequest, "open.failed", err.Error())
		return
	}
	writeJSON(w, l.chosen())
}

func (l *Local) inventory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "method", "GET expected")
		return
	}
	snap, err := l.Session.Snapshot()
	if err != nil {
		fail(w, http.StatusConflict, "not.scanned", err.Error())
		return
	}
	writeJSON(w, snap.Inventory)
}

// The scan streams because it walks tens of thousands of files: a request that
// answers only at the end looks the same as one that has hung.
func (l *Local) scan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "method", "POST expected")
		return
	}

	stream := newStream(w)
	snap, err := l.Session.Read(stream.sink())
	if err != nil {
		stream.finish(nil, err)
		return
	}
	stream.finish(snap.Inventory, nil)
}

func (l *Local) chosen() Chosen {
	c := Chosen{Root: l.Session.Root()}
	if snap, err := l.Session.Snapshot(); err == nil {
		c.Scanned = true
		c.ScannedAt = snap.At.Format(time.RFC3339)
		live := snap.Install.Liveness()
		c.Running, c.ActiveWorld = live.ServerRunning, live.ActiveWorld
	}
	return c
}

func (l *Local) state(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "method", "GET expected")
		return
	}
	s, err := l.State()
	if err != nil {
		fail(w, http.StatusInternalServerError, "scan.failed", err.Error())
		return
	}
	writeJSON(w, s)
}

func (l *Local) plan(w http.ResponseWriter, r *http.Request) {
	var req PlanRequest
	if !readJSON(w, r) || !decode(w, r, &req) {
		return
	}
	p, err := l.Plan(req)
	if err != nil {
		fail(w, http.StatusInternalServerError, "plan.failed", err.Error())
		return
	}
	writeJSON(w, p)
}

func (l *Local) verify(w http.ResponseWriter, r *http.Request) {
	var req VerifyRequest
	if !readJSON(w, r) || !decode(w, r, &req) {
		return
	}
	res, err := l.Verify(r.Context(), req)
	if err != nil {
		fail(w, http.StatusBadRequest, "verify.failed", err.Error())
		return
	}
	writeJSON(w, res)
}

// One transfer at a time, because two runs over the same installation would
// interleave their reads and produce a copy of no single moment.
func (l *Local) run(w http.ResponseWriter, r *http.Request) {
	var req RunRequest
	if !readJSON(w, r) || !decode(w, r, &req) {
		return
	}
	if !l.mu.TryLock() {
		fail(w, http.StatusConflict, "run.busy", "a transfer is already running")
		return
	}
	defer l.mu.Unlock()

	s := newStream(w)
	s.finish(l.Run(r.Context(), req, s.sink()))
}
