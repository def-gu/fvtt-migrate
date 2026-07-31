package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/scan"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "scan" {
		fmt.Fprintln(os.Stderr, "usage: fvtt-migrate scan --root <FoundryVTT user data> [--core <application dir>]")
		os.Exit(2)
	}

	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	root := fs.String("root", "", "Foundry user-data directory (the one holding Config and Data)")
	core := fs.String("core", "", "Foundry application directory, to recognise built-in assets")
	limit := fs.Int("limit", 10, "how many entries to show in each list")
	fs.Parse(os.Args[2:])

	if *root == "" {
		fs.Usage()
		os.Exit(2)
	}
	if err := run(*root, *core, *limit); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(root, core string, limit int) error {
	inst, err := foundry.Open(root)
	if err != nil {
		return err
	}
	inv, err := inst.Inventory()
	if err != nil {
		return err
	}

	coreDir := ""
	if core != "" {
		coreDir = core + "/public"
	}
	ix, err := scan.Build(inst.Data, coreDir)
	if err != nil {
		return err
	}
	sum, err := scan.Analyze(inv, ix)
	if err != nil {
		return err
	}

	report(inst, inv, ix, sum, limit)
	return nil
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
