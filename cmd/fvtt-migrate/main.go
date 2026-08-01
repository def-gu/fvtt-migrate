package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/def-gu/fvtt-migrate/internal/content"
	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/plan"
	"github.com/def-gu/fvtt-migrate/internal/scan"
	"github.com/def-gu/fvtt-migrate/internal/transfer"
	"github.com/def-gu/fvtt-migrate/internal/upstream"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	root := fs.String("root", "", "Foundry user-data directory (the one holding Config and Data)")
	core := fs.String("core", "", "Foundry application directory, to recognise built-in assets")
	limit := fs.Int("limit", 10, "how many entries to show in each list")
	out := fs.String("out", "plan.yaml", "where to write the plan")
	targetCore := fs.String("target-core", "", "Foundry version the plan targets (default: highest found)")
	checkUpdates := fs.Bool("check-updates", false, "read upstream manifests; without it nothing leaves this machine")

	to := fs.String("to", "", "directory to migrate into")
	force := fs.Bool("force", false, "copy even while a world is loaded")

	switch cmd {
	case "scan", "plan", "apply":
		fs.Parse(os.Args[2:])
	default:
		usage()
	}
	if *root == "" {
		fs.Usage()
		os.Exit(2)
	}

	var err error
	switch cmd {
	case "scan":
		err = runScan(*root, *core, *limit)
	case "plan":
		err = runPlan(*root, *core, *targetCore, *out, *checkUpdates)
	case "apply":
		if *to == "" {
			fs.Usage()
			os.Exit(2)
		}
		err = runApply(*root, *core, *out, *to, *force)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runApply(root, core, planPath, to string, force bool) error {
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

	if live := inst.Liveness(); live.ActiveWorld != "" && !force {
		return fmt.Errorf("world %q is loaded in Foundry; copying an open database corrupts it. "+
			"Return Foundry to setup, or pass --force to copy anyway", live.ActiveWorld)
	} else if live.ServerRunning {
		fmt.Fprintln(os.Stderr, "note: Foundry is running with no world loaded, which is safe to copy")
	}

	sel, err := transfer.Select(p, inst.Data, ix, sum)
	if err != nil {
		return err
	}
	fmt.Printf("Selected %d files, %s\n", len(sel.Paths), humanBytes(sel.Bytes))

	cache := content.OpenCache(inst.Root)
	hashed := content.HashTree(inst.Data, sel.Paths, cache, 0)
	if err := cache.Save(); err != nil {
		return err
	}
	if len(hashed.Errors) > 0 {
		for rel, e := range hashed.Errors {
			fmt.Fprintf(os.Stderr, "  unreadable: %s: %v\n", rel, e)
		}
		return fmt.Errorf("%d files could not be read", len(hashed.Errors))
	}
	fmt.Printf("Hashed %d files, reused %d cached digests\n", hashed.Hashed, hashed.Reused)

	prog, err := transfer.Apply(context.Background(), inst.Data, sel, hashed.Entries,
		transfer.NewFileTarget(to), transfer.Options{})
	if err != nil {
		return err
	}

	if moved := content.Recheck(inst.Data, hashed.Entries); len(moved) > 0 {
		for _, rel := range moved[:min(len(moved), 10)] {
			fmt.Fprintf(os.Stderr, "  changed while copying: %s\n", rel)
		}
		return fmt.Errorf("%d source files changed during the copy; the result is not a snapshot of any single moment", len(moved))
	}

	fmt.Printf("\nUnique blobs   %d\n", prog.Negotiated)
	fmt.Printf("Transferred    %d blobs, %s\n", prog.Uploaded, humanBytes(prog.UploadedByte))
	fmt.Printf("Already there  %d blobs, %s not sent\n", prog.Skipped, humanBytes(prog.SkippedByte))
	fmt.Printf("Placed         %d files under %s\n", prog.Placed, to)
	fmt.Println("\nThe source installation was not modified.")
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  fvtt-migrate scan --root <user data> [--core <application dir>]")
	fmt.Fprintln(os.Stderr, "  fvtt-migrate plan --root <user data> [--core <application dir>] [--out plan.yaml] [--check-updates]")
	fmt.Fprintln(os.Stderr, "  fvtt-migrate apply --root <user data> --to <directory> [--out plan.yaml]")
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

func runScan(root, core string, limit int) error {
	inst, inv, ix, sum, err := analyse(root, core)
	if err != nil {
		return err
	}
	report(inst, inv, ix, sum, limit)
	return nil
}

func runPlan(root, core, targetCore, out string, checkUpdates bool) error {
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

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := p.Write(f); err != nil {
		return err
	}

	reportPlan(p, out, checkUpdates)
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

func report(inst *foundry.Install, inv *foundry.Inventory, ix *scan.Index, s *scan.Summary, limit int) {
	fmt.Printf("Installation  %s\n", inst.Root)
	fmt.Printf("Contents      %d worlds, %d systems, %d modules, %d files\n\n",
		len(inv.Worlds), len(inv.Systems), len(inv.Modules), ix.Len())

	installed := map[string]bool{}
	for _, sys := range inv.Systems {
		installed[sys.ID] = true
	}
	fmt.Println("Worlds")
	for _, w := range inv.Worlds {
		note := ""
		if !installed[w.System] {
			note = "  [system not installed]"
		}
		fmt.Printf("  %-36s %-16s core %-8s%s\n", w.ID, w.System+"@"+w.SystemVersion, w.CoreVersion, note)
	}

	fmt.Printf("\nAssets\n")
	fmt.Printf("  referenced by documents   %6d files  %10s\n", s.Referenced.Files, humanBytes(s.Referenced.Bytes))
	fmt.Printf("  inside packages           %6d files  %10s\n", s.Packaged.Files, humanBytes(s.Packaged.Bytes))
	fmt.Printf("  orphaned                  %6d files  %10s\n", s.Orphans.Files, humanBytes(s.Orphans.Bytes))
	fmt.Printf("  built into Foundry        %6d refs   (not transferred)\n", s.CoreRefs)
	fmt.Printf("  transfer total            %6d files  %10s\n",
		s.Referenced.Files+s.Packaged.Files, humanBytes(s.Referenced.Bytes+s.Packaged.Bytes))

	if len(s.OrphansByDir) > 0 {
		fmt.Printf("\nOrphaned by directory\n")
		type row struct {
			dir string
			b   scan.Bucket
		}
		rows := make([]row, 0, len(s.OrphansByDir))
		for d, b := range s.OrphansByDir {
			rows = append(rows, row{d, b})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].b.Bytes > rows[j].b.Bytes })
		for i, r := range rows {
			if i >= limit {
				fmt.Printf("  ... and %d more\n", len(rows)-limit)
				break
			}
			note := ""
			if n := s.BrokenByDir[r.dir]; n > 0 {
				note = fmt.Sprintf("  <- %d broken refs point here", n)
			}
			fmt.Printf("  %-30s %6d files  %10s%s\n", r.dir, r.b.Files, humanBytes(r.b.Bytes), note)
		}
	}

	if renamed := s.Renamed(); len(renamed) > 0 {
		fmt.Printf("\nLikely renamed, not junk: %v\n", renamed)
		fmt.Println("  These directories hold unreferenced files while broken references")
		fmt.Println("  point into them. Skipping them would migrate broken scenes.")
	}

	printMissing("Broken references", s.Broken, limit)
	printMissing("Case-only matches (break on a Linux server)", s.CaseIssues, limit)

	if len(inv.Problems) > 0 {
		fmt.Printf("\nUnreadable manifests\n")
		for _, p := range inv.Problems {
			fmt.Printf("  %s: %s\n", p.Dir, p.Reason)
		}
	}
	fmt.Println("\nNothing was modified.")
}

func printMissing(title string, list []scan.Missing, limit int) {
	if len(list) == 0 {
		return
	}
	fmt.Printf("\n%s: %d\n", title, len(list))
	for i, m := range list {
		if i >= limit {
			fmt.Printf("  ... and %d more\n", len(list)-limit)
			break
		}
		fmt.Printf("  %-64s %4d refs  (%s)\n", truncate(m.Path, 64), m.Refs, m.Where)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n+3:]
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
