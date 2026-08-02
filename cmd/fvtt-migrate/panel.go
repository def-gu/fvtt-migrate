package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/def-gu/fvtt-migrate/internal/api"
	"github.com/def-gu/fvtt-migrate/internal/content"
	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/plan"
	"github.com/def-gu/fvtt-migrate/internal/progress"
	"github.com/def-gu/fvtt-migrate/internal/report"
	"github.com/def-gu/fvtt-migrate/internal/session"
	"github.com/def-gu/fvtt-migrate/internal/transfer"
	"github.com/def-gu/fvtt-migrate/internal/upstream"
	"github.com/def-gu/fvtt-migrate/internal/verify"
)

func runPanel(root, core, listen string) error {
	if root == "" {
		fmt.Println("Installation   none found, choose one in the panel")
	} else {
		fmt.Printf("Installation   %s\n", root)
	}
	fmt.Printf("Panel          http://%s/\n\n", listen)
	fmt.Println("Open that address in a browser. Close this window to stop.")
	return servePanel(root, core, listen)
}

func servePanel(root, core, listen string) error {
	if !api.LocalOnly(listen) {
		return fmt.Errorf("the panel has no password, so it is only served on this machine. Use --listen 127.0.0.1:7788")
	}
	sess := session.New(core)
	if root != "" {
		if _, err := sess.Open(root); err != nil {
			return err
		}
	}

	local := &api.Local{
		Session: sess,
		State:   func() (*api.State, error) { return panelState(sess) },
		Plan:    func(req api.PlanRequest) (*plan.Plan, error) { return panelPlan(sess, req) },
		Run: func(ctx context.Context, req api.RunRequest, s progress.Sink) (any, error) {
			return panelRun(ctx, root, core, req, s)
		},
		Verify: func(ctx context.Context, req api.VerifyRequest) (*verify.Result, error) {
			return panelVerify(ctx, root, core, req)
		},
	}

	mux := http.NewServeMux()
	local.Routes(mux)
	page, err := api.Panel()
	if err != nil {
		return err
	}
	mux.Handle("/", page)

	return api.Listener(listen, mux).ListenAndServe()
}

func panelState(sess *session.Session) (*api.State, error) {
	snap, err := sess.Snapshot()
	if err != nil {
		if snap, err = sess.Read(nil); err != nil {
			return nil, err
		}
	}
	return &api.State{Root: snap.Install.Root, Targets: report.TargetVersions(snap.Inv), Scan: snap.Scan}, nil
}

// Built from the reading already in memory. Rebuilding a plan is a decision
// over known facts, and re-walking the disk for every changed option was what
// made the panel look frozen.
func panelPlan(sess *session.Session, req api.PlanRequest) (*plan.Plan, error) {
	snap, err := sess.Snapshot()
	if err != nil {
		return nil, err
	}

	opts := plan.Options{TargetCore: req.TargetCore}
	if req.CheckUpdates {
		pkgs := append(append([]foundry.Package{}, snap.Inv.Systems...), snap.Inv.Modules...)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		opts.Updates = upstream.New().CheckAll(ctx, pkgs)
	}
	return plan.Build(snap.Install, snap.Inv, snap.Summary, opts), nil
}

func panelRun(ctx context.Context, root, core string, req api.RunRequest, sink progress.Sink) (any, error) {
	if req.Plan == nil {
		return nil, fmt.Errorf("no plan was sent")
	}
	if findings := req.Plan.Validate(); plan.HasErrors(findings) {
		return nil, fmt.Errorf("%s", findings[0].Message)
	}
	if isRemote(req.To) {
		if err := api.CheckAddress(req.To, false); err != nil {
			return nil, err
		}
		if req.Token == "" {
			return nil, fmt.Errorf("the receiving side needs its access key")
		}
		greet, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if _, err := api.NewClient(req.To, req.Token).Hello(greet); err != nil {
			return nil, err
		}
	}

	inst, _, ix, sum, err := analyse(root, core)
	if err != nil {
		return nil, err
	}
	if live := inst.Liveness(); live.ActiveWorld != "" {
		return nil, fmt.Errorf("world %q is open in Foundry. Return Foundry to the setup screen first", live.ActiveWorld)
	}

	sel, err := transfer.Select(req.Plan, inst.Data, ix, sum)
	if err != nil {
		return nil, err
	}

	cache := content.OpenCache(inst.Root)
	hashed := content.HashTreeWithProgress(inst.Data, sel.Paths, cache, 0, sink)
	if err := cache.Save(); err != nil {
		return nil, err
	}
	if len(hashed.Errors) > 0 {
		return nil, fmt.Errorf("%d files could not be read", len(hashed.Errors))
	}

	prog, err := transfer.Apply(ctx, inst.Data, sel, hashed.Entries,
		newTarget(req.To, req.Token), transfer.Options{Sink: sink, DryRun: req.DryRun})
	if err != nil {
		return nil, err
	}
	if moved := content.Recheck(inst.Data, hashed.Entries); len(moved) > 0 {
		return nil, fmt.Errorf("%d files changed while being read. Close Foundry and run again", len(moved))
	}
	return prog, nil
}

func panelVerify(ctx context.Context, root, core string, req api.VerifyRequest) (*verify.Result, error) {
	inst, err := foundry.Open(root)
	if err != nil {
		return nil, err
	}

	if isRemote(req.To) {
		reported, err := api.NewClient(req.To, req.Token).Worlds(ctx)
		if err != nil {
			return nil, err
		}
		counts := map[string]int{}
		for _, w := range reported {
			counts[w.ID] = w.Documents
		}
		worlds, err := verify.Remote(inst.Data, counts)
		if err != nil {
			return nil, err
		}
		return &verify.Result{Missing: []string{}, Mismatch: []string{}, Worlds: worlds}, nil
	}

	worlds, err := verify.Worlds(inst.Data, req.To)
	if err != nil {
		return nil, err
	}
	return &verify.Result{Missing: []string{}, Mismatch: []string{}, Worlds: worlds}, nil
}
