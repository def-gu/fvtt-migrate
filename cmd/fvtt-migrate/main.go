package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/def-gu/fvtt-migrate/internal/api"
	"github.com/def-gu/fvtt-migrate/internal/content"
	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/plan"
	"github.com/def-gu/fvtt-migrate/internal/progress"
	"github.com/def-gu/fvtt-migrate/internal/report"
	"github.com/def-gu/fvtt-migrate/internal/scan"
	"github.com/def-gu/fvtt-migrate/internal/transfer"
	"github.com/def-gu/fvtt-migrate/internal/upstream"
	"github.com/def-gu/fvtt-migrate/internal/verify"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		if err := runLaunch(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	if os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h" {
		usage()
	}
	if os.Args[1] == "version" {
		fmt.Println("fvtt-migrate", version)
		return
	}

	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	root := fs.String("root", "", "Foundry user-data directory (the one holding Config and Data); found automatically when omitted")
	core := fs.String("core", "", "Foundry application directory, to recognise built-in assets")
	limit := fs.Int("limit", 10, "how many entries to show in each list")
	out := fs.String("out", "plan.yaml", "where to write the plan")
	targetCore := fs.String("target-core", "", "Foundry version the plan targets (default: highest found)")
	checkUpdates := fs.Bool("check-updates", false, "read upstream manifests; without it nothing leaves this machine")

	to := fs.String("to", "", "directory to migrate into")
	force := fs.Bool("force", false, "copy even while a world is loaded")
	asJSON := fs.Bool("json", false, "emit machine-readable output instead of text")
	dryRun := fs.Bool("dry-run", false, "work out what would be transferred without writing anything")
	listen := fs.String("listen", "127.0.0.1:7788", "address the receiving side listens on")
	token := fs.String("token", "", "shared token; prefer the FVTT_MIGRATE_TOKEN variable, since arguments are visible to other users")
	insecure := fs.Bool("insecure", false, "allow plain HTTP to a remote host")
	deep := fs.Bool("deep", false, "re-hash every transferred file at the target")

	switch cmd {
	case "scan", "plan", "apply", "verify", "serve", "panel":
		fs.Parse(os.Args[2:])
	default:
		usage()
	}
	if cmd == "panel" {
		at := *root
		if at == "" {
			inst, err := foundry.Discover()
			if err != nil {
				fmt.Fprintln(os.Stderr, "error: no Foundry installation found; pass --root")
				os.Exit(1)
			}
			at = inst.Root
		}
		if err := runPanel(at, *core, *listen); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	if cmd == "serve" {
		if *to == "" {
			fs.Usage()
			os.Exit(2)
		}
		if err := runServe(*to, *listen, tokenFrom(*token)); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	if *root == "" {
		fs.Usage()
		os.Exit(2)
	}

	var err error
	switch cmd {
	case "scan":
		err = runScan(*root, *core, *limit, *asJSON)
	case "plan":
		err = runPlan(*root, *core, *targetCore, *out, *checkUpdates, *asJSON)
	case "apply":
		if *to == "" {
			fs.Usage()
			os.Exit(2)
		}
		err = runApply(*root, *core, *out, *to, tokenFrom(*token), *force, *asJSON, *dryRun, *insecure)
	case "verify":
		if *to == "" {
			fs.Usage()
			os.Exit(2)
		}
		err = runVerify(*root, *core, *out, *to, tokenFrom(*token), *deep, *asJSON, *insecure)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runApply(root, core, planPath, to, token string, force, asJSON, dryRun, insecure bool) error {
	inst, _, ix, sum, err := analyse(root, core)
	if err != nil {
		return err
	}

	f, err := os.Open(planPath)
	if err != nil {
		return err
	}
	p, err := plan.Read(f)
	f.Close()
	if err != nil {
		return err
	}

	if isRemote(to) {
		if err := api.CheckAddress(to, insecure); err != nil {
			return err
		}
		if token == "" {
			return fmt.Errorf("a token is required for a remote target; set FVTT_MIGRATE_TOKEN")
		}
	}

	findings := p.Validate()
	for _, f := range findings {
		fmt.Fprintf(os.Stderr, "  %s [%s] %s: %s\n", f.Severity, f.Code, f.Where, f.Message)
	}
	if plan.HasErrors(findings) {
		return fmt.Errorf("the plan has errors; fix them and run again")
	}

	sink := progressSink(asJSON)
	if live := inst.Liveness(); live.ActiveWorld != "" && !force {
		return fmt.Errorf("world %q is loaded in Foundry; copying an open database corrupts it. "+
			"Return Foundry to setup, or pass --force to copy anyway", live.ActiveWorld)
	} else if live.ServerRunning {
		progress.Note(sink, "foundry.running.no_world",
			"Foundry is running with no world loaded, which is safe to copy", nil)
	}

	sel, err := transfer.Select(p, inst.Data, ix, sum)
	if err != nil {
		return err
	}
	if !asJSON {
		fmt.Printf("Selected %d files, %s\n", len(sel.Paths), report.Bytes(sel.Bytes))
	}

	cache := content.OpenCache(inst.Root)
	hashed := content.HashTreeWithProgress(inst.Data, sel.Paths, cache, 0, sink)
	if err := cache.Save(); err != nil {
		return err
	}
	if len(hashed.Errors) > 0 {
		for rel, e := range hashed.Errors {
			fmt.Fprintf(os.Stderr, "  unreadable: %s: %v\n", rel, e)
		}
		return fmt.Errorf("%d files could not be read", len(hashed.Errors))
	}
	if !asJSON {
		fmt.Printf("Hashed %d files, reused %d cached digests\n", hashed.Hashed, hashed.Reused)
	}

	prog, err := transfer.Apply(context.Background(), inst.Data, sel, hashed.Entries,
		newTarget(to, token), transfer.Options{Sink: sink, DryRun: dryRun})
	if err != nil {
		return err
	}

	if moved := content.Recheck(inst.Data, hashed.Entries); len(moved) > 0 {
		for _, rel := range moved[:min(len(moved), 10)] {
			fmt.Fprintf(os.Stderr, "  changed while copying: %s\n", rel)
		}
		return fmt.Errorf("%d source files changed during the copy; the result is not a snapshot of any single moment", len(moved))
	}

	if asJSON {
		return report.JSON(os.Stdout, prog)
	}

	if dryRun {
		fmt.Printf("\nWould transfer %d blobs, %s\n", prog.WouldSend, report.Bytes(prog.WouldSendByte))
		fmt.Printf("Already there  %d blobs, %s\n", prog.Skipped, report.Bytes(prog.SkippedByte))
		fmt.Println("\nNothing was written. Drop --dry-run to do it.")
		return nil
	}

	fmt.Printf("\nUnique blobs   %d\n", prog.Negotiated)
	fmt.Printf("Transferred    %d blobs, %s\n", prog.Uploaded, report.Bytes(prog.UploadedByte))
	fmt.Printf("Already there  %d blobs, %s not sent\n", prog.Skipped, report.Bytes(prog.SkippedByte))
	fmt.Printf("Placed         %d files under %s\n", prog.Placed, to)
	fmt.Println("\nThe source installation was not modified.")
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  fvtt-migrate scan --root <user data> [--core <application dir>]")
	fmt.Fprintln(os.Stderr, "  fvtt-migrate plan --root <user data> [--core <application dir>] [--out plan.yaml] [--check-updates]")
	fmt.Fprintln(os.Stderr, "  fvtt-migrate apply --root <user data> --to <directory> [--out plan.yaml] [--dry-run]")
	fmt.Fprintln(os.Stderr, "  fvtt-migrate verify --root <user data> --to <directory> [--deep]")
	fmt.Fprintln(os.Stderr, "  fvtt-migrate panel --root <user data> [--listen 127.0.0.1:7788]")
	fmt.Fprintln(os.Stderr, "  fvtt-migrate serve --to <directory> [--listen 127.0.0.1:7788]")
	fmt.Fprintln(os.Stderr, "  fvtt-migrate version")
	os.Exit(2)
}

func analyse(root, core string) (*foundry.Install, *foundry.Inventory, *scan.Index, *scan.Summary, error) {
	inst, err := foundry.Open(root)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	inv, err := inst.Inventory()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	coreDir := ""
	if core != "" {
		coreDir = core + "/public"
	}
	ix, err := scan.Build(inst.Data, coreDir)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	sum, err := scan.Analyze(inv, ix)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return inst, inv, ix, sum, nil
}

func runScan(root, core string, limit int, asJSON bool) error {
	inst, inv, ix, sum, err := analyse(root, core)
	if err != nil {
		return err
	}
	r := report.BuildScan(inst, inv, ix, sum)
	if asJSON {
		return report.JSON(os.Stdout, r)
	}
	r.Text(os.Stdout, limit)
	return nil
}

func runPlan(root, core, targetCore, out string, checkUpdates, asJSON bool) error {
	inst, inv, _, sum, err := analyse(root, core)
	if err != nil {
		return err
	}

	opts := plan.Options{TargetCore: targetCore}
	if checkUpdates {
		pkgs := append(append([]foundry.Package{}, inv.Systems...), inv.Modules...)
		fmt.Fprintf(os.Stderr, "reading %d upstream manifests...\n", len(pkgs))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		opts.Updates = upstream.New().CheckAll(ctx, pkgs)
	}

	p := plan.Build(inst, inv, sum, opts)

	if asJSON {
		if err := report.JSON(os.Stdout, p); err != nil {
			return err
		}
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := p.Write(f); err != nil {
		return err
	}

	if !asJSON {
		reportPlan(p, out, checkUpdates)
	}
	return nil
}

func reportPlan(p *plan.Plan, out string, checked bool) {
	bySource := map[plan.Source]int{}
	byAdvice := map[plan.Recommendation][]plan.Package{}
	for _, pkg := range p.Packages {
		bySource[pkg.Source]++
		byAdvice[pkg.Recommend] = append(byAdvice[pkg.Recommend], pkg)
	}

	fmt.Printf("Wrote %s\n\n", out)
	fmt.Printf("Target Foundry %s\n", p.Source.TargetCore)
	fmt.Printf("Packages       %d from manifest, %d to upload, %d already on target\n\n",
		bySource[plan.FromManifest], bySource[plan.FromUpload], bySource[plan.FromCache])
	fmt.Printf("Worlds         %d included, %d blocked\n", countIncluded(p.Worlds), len(p.Worlds)-countIncluded(p.Worlds))
	for _, w := range p.Worlds {
		if w.Blocker != "" {
			fmt.Printf("  blocked: %-34s %s\n", w.ID, w.Blocker)
		}
	}

	if !checked {
		fmt.Println("\nUpstream was not contacted. Re-run with --check-updates to see what is available.")
	} else {
		section("Must be updated to run on the target", byAdvice[plan.Required], true)
		section("Worth taking: still works now and is ready for the next generation", widening(byAdvice[plan.Upgrade]), true)
		section("Updates that would drop the target version, left pinned", byAdvice[plan.Keep], false)
		section("Nothing available runs on the target", byAdvice[plan.Blocked], true)
	}

	manual := manualCheck(p.Packages)
	if len(manual) > 0 {
		fmt.Printf("\nNeeds a human eye: %d\n", len(manual))
		for _, pkg := range manual {
			fmt.Printf("  %-34s %-10s %s\n", pkg.ID, pkg.Version, pkg.Reason)
		}
	}

	fmt.Println("\nReview and edit the plan before applying it. Nothing was modified.")
}

func section(title string, pkgs []plan.Package, detailed bool) {
	if len(pkgs) == 0 {
		return
	}
	fmt.Printf("\n%s: %d\n", title, len(pkgs))
	if !detailed {
		var names []string
		for _, p := range pkgs {
			names = append(names, p.ID)
		}
		fmt.Printf("  %s\n", strings.Join(names, ", "))
		return
	}
	for _, p := range pkgs {
		fmt.Printf("  %-34s %-10s -> %-10s (%s)\n", p.ID, p.Version, p.Available, p.CompatAvailable)
	}
}

func widening(pkgs []plan.Package) []plan.Package {
	var out []plan.Package
	for _, p := range pkgs {
		if p.Widens {
			out = append(out, p)
		}
	}
	return out
}

func manualCheck(pkgs []plan.Package) []plan.Package {
	var out []plan.Package
	for _, p := range pkgs {
		if p.Source == plan.FromUpload && !p.Premium {
			out = append(out, p)
		}
	}
	return out
}

func countIncluded(ws []plan.World) int {
	n := 0
	for _, w := range ws {
		if w.Include {
			n++
		}
	}
	return n
}

func runVerify(root, core, planPath, to, token string, deep, asJSON, insecure bool) error {
	if isRemote(to) {
		return verifyRemote(root, core, to, token, asJSON, insecure)
	}
	return verifyLocal(root, core, planPath, to, deep, asJSON)
}

func verifyRemote(root, core, to, token string, asJSON, insecure bool) error {
	if err := api.CheckAddress(to, insecure); err != nil {
		return err
	}
	inst, err := foundry.Open(root)
	if err != nil {
		return err
	}
	_ = core

	reported, err := api.NewClient(to, token).Worlds(context.Background())
	if err != nil {
		return err
	}
	counts := map[string]int{}
	for _, w := range reported {
		counts[w.ID] = w.Documents
	}

	worlds, err := verify.Remote(inst.Data, counts)
	if err != nil {
		return err
	}
	res := &verify.Result{Missing: []string{}, Mismatch: []string{}, Worlds: worlds}

	if asJSON {
		if err := report.JSON(os.Stdout, res); err != nil {
			return err
		}
		if !res.OK() {
			return fmt.Errorf("verification failed")
		}
		return nil
	}

	fmt.Printf("Checked %s\n\nWorlds\n", to)
	for _, w := range worlds {
		status := "ok"
		if !w.OK() {
			status = "MISMATCH"
		}
		fmt.Printf("  %-34s source=%-7d target=%-7d %s\n", w.ID, w.SourceDocs, w.TargetDocs, status)
	}
	if !res.OK() {
		return fmt.Errorf("verification failed")
	}
	fmt.Println("\nEvery world that travelled reads back with the same number of documents.")
	return nil
}

func verifyLocal(root, core, planPath, to string, deep, asJSON bool) error {
	inst, _, ix, sum, err := analyse(root, core)
	if err != nil {
		return err
	}

	f, err := os.Open(planPath)
	if err != nil {
		return err
	}
	p, err := plan.Read(f)
	f.Close()
	if err != nil {
		return err
	}

	sel, err := transfer.Select(p, inst.Data, ix, sum)
	if err != nil {
		return err
	}
	cache := content.OpenCache(inst.Root)
	expected := content.HashTreeWithProgress(inst.Data, sel.Paths, cache, 0, progressSink(asJSON)).Entries
	if err := cache.Save(); err != nil {
		return err
	}

	res, err := verify.Files(to, expected, deep)
	if err != nil {
		return err
	}
	res.Worlds, err = verify.Worlds(inst.Data, to)
	if err != nil {
		return err
	}

	if asJSON {
		if err := report.JSON(os.Stdout, res); err != nil {
			return err
		}
		if !res.OK() {
			return fmt.Errorf("verification failed")
		}
		return nil
	}

	fmt.Printf("Checked %d files at %s\n", len(expected), to)
	if deep {
		fmt.Printf("  re-hashed %d\n", res.Rehashed)
	}
	report := func(title string, list []string) {
		if len(list) == 0 {
			return
		}
		fmt.Printf("\n%s: %d\n", title, len(list))
		for i, rel := range list {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(list)-10)
				break
			}
			fmt.Printf("  %s\n", rel)
		}
	}
	report("Missing at the target", res.Missing)
	report("Contents differ", res.Mismatch)

	fmt.Printf("\nWorlds\n")
	for _, w := range res.Worlds {
		status := "ok"
		if !w.OK() {
			status = "MISMATCH " + fmt.Sprint(w.Namespaces)
			if w.Err != nil {
				status = "unreadable: " + w.Err.Error()
			}
		}
		fmt.Printf("  %-34s source=%-7d target=%-7d %s\n", w.ID, w.SourceDocs, w.TargetDocs, status)
	}

	if !res.OK() {
		return fmt.Errorf("verification failed")
	}
	fmt.Println("\nEverything the plan selected is present and reads back identically.")
	return nil
}

// Machine-readable runs stream events as JSON lines on stderr, leaving stdout
// for the single result document. Terminal runs get one rewriting line.
func progressSink(asJSON bool) progress.Sink {
	switch {
	case asJSON:
		return progress.Lines(os.Stderr)
	case progress.IsTerminal(os.Stderr):
		return progress.Ticker(os.Stderr)
	default:
		return progress.Plain(os.Stderr)
	}
}

func isRemote(to string) bool {
	return strings.HasPrefix(to, "http://") || strings.HasPrefix(to, "https://")
}

func newTarget(to, token string) transfer.Target {
	if isRemote(to) {
		return api.NewClient(to, token)
	}
	return transfer.NewFileTarget(to)
}

func tokenFrom(flag string) string {
	if v := os.Getenv("FVTT_MIGRATE_TOKEN"); v != "" {
		return v
	}
	return flag
}

func runServe(dir, listen, token string) error {
	stored := false
	if token == "" {
		var fresh bool
		var err error
		token, fresh, err = api.StoredToken(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "note: the key could not be saved, so the next start will make a new one:", err)
		}
		stored = !fresh
	}
	local := api.LocalOnly(listen)
	handler, err := api.Handler(transfer.NewReceiver(dir), api.ServeOptions{Root: dir, Token: token, Panel: local})
	if err != nil {
		return err
	}

	fmt.Printf("Receiving into %s\n", dir)
	if local {
		fmt.Printf("Panel          http://%s/\n", listen)
	} else {
		fmt.Printf("Panel          not served: %s is reachable from elsewhere\n", listen)
	}
	if stored {
		fmt.Printf("Token          %s  (the same key as last time)\n\n", token)
	} else {
		fmt.Printf("Token          %s\n\n", token)
	}
	fmt.Println("On the sending machine:")
	fmt.Printf("  export FVTT_MIGRATE_TOKEN=%s\n", token)
	if local {
		fmt.Printf("  fvtt-migrate apply --root <user data> --to http://%s\n", listen)
	} else {
		fmt.Println("  fvtt-migrate apply --root <user data> --to https://<the address your proxy serves>")
	}

	return api.Listener(listen, handler).ListenAndServe()
}
