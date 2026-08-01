package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/def-gu/fvtt-migrate/internal/plan"
	"github.com/def-gu/fvtt-migrate/internal/progress"
	"github.com/def-gu/fvtt-migrate/internal/report"
	"github.com/def-gu/fvtt-migrate/internal/verify"
)

const (
	PathPing   = "/api/ping"
	PathState  = "/api/state"
	PathPlan   = "/api/plan"
	PathRun    = "/api/run"
	PathVerify = "/api/verify"
)

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
	mux.HandleFunc(PathState, l.state)
	mux.HandleFunc(PathPlan, l.plan)
	mux.HandleFunc(PathRun, l.run)
	mux.HandleFunc(PathVerify, l.verify)
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

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	flusher, _ := w.(http.Flusher)
	var mu sync.Mutex

	sink := progress.Throttle(progress.Func(func(v any) {
		mu.Lock()
		defer mu.Unlock()
		enc.Encode(v)
		if flusher != nil {
			flusher.Flush()
		}
	}), 100*time.Millisecond)

	result, err := l.Run(r.Context(), req, sink)

	mu.Lock()
	defer mu.Unlock()
	if err != nil {
		enc.Encode(map[string]any{"type": "failed", "message": err.Error()})
	} else {
		enc.Encode(map[string]any{"type": "result", "result": result})
	}
	if flusher != nil {
		flusher.Flush()
	}
}
