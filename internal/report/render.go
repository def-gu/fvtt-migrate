package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (s *Scan) Text(w io.Writer, limit int) {
	fmt.Fprintf(w, "Installation  %s\n", s.Root)
	fmt.Fprintf(w, "Contents      %d worlds, %d systems, %d modules, %d files\n\n",
		s.Counts.Worlds, s.Counts.Systems, s.Counts.Modules, s.Counts.Files)

	fmt.Fprintln(w, "Worlds")
	for _, world := range s.Worlds {
		note := ""
		if !world.SystemInstalled {
			note = "  [system not installed]"
		}
		fmt.Fprintf(w, "  %-36s %-16s core %-8s%s\n",
			world.ID, world.System+"@"+world.SystemVersion, world.CoreVersion, note)
	}

	a := s.Assets
	fmt.Fprintf(w, "\nAssets\n")
	fmt.Fprintf(w, "  referenced by documents   %6d files  %10s\n", a.Referenced.Files, Bytes(a.Referenced.Bytes))
	fmt.Fprintf(w, "  inside packages           %6d files  %10s\n", a.Packaged.Files, Bytes(a.Packaged.Bytes))
	fmt.Fprintf(w, "  orphaned                  %6d files  %10s\n", a.Orphaned.Files, Bytes(a.Orphaned.Bytes))
	fmt.Fprintf(w, "  built into Foundry        %6d refs   (not transferred)\n", a.CoreRefs)
	fmt.Fprintf(w, "  transfer total            %6d files  %10s\n",
		a.Referenced.Files+a.Packaged.Files, Bytes(a.Referenced.Bytes+a.Packaged.Bytes))

	if len(a.Directories) > 0 {
		fmt.Fprintf(w, "\nOrphaned by directory\n")
		for i, d := range a.Directories {
			if i >= limit {
				fmt.Fprintf(w, "  ... and %d more\n", len(a.Directories)-limit)
				break
			}
			note := ""
			if d.Broken > 0 {
				note = fmt.Sprintf("  <- %d broken refs point here", d.Broken)
			}
			fmt.Fprintf(w, "  %-30s %6d files  %10s%s\n", d.Path, d.Files, Bytes(d.Bytes), note)
		}
	}

	var stale []string
	for _, d := range a.Directories {
		if d.Stale {
			stale = append(stale, d.Path)
		}
	}
	if len(stale) > 0 {
		fmt.Fprintf(w, "\nLikely renamed, not junk: %v\n", stale)
		fmt.Fprintln(w, "  These directories hold unreferenced files while broken references")
		fmt.Fprintln(w, "  point into them. Skipping them would migrate broken scenes.")
	}

	missing(w, "Broken references", s.Broken, limit)
	missing(w, "Case-only matches (break on a Linux server)", s.CaseOnly, limit)

	if len(s.Problems) > 0 {
		fmt.Fprintf(w, "\nUnreadable manifests\n")
		for _, p := range s.Problems {
			fmt.Fprintf(w, "  %s: %s\n", p.Dir, p.Reason)
		}
	}
	fmt.Fprintln(w, "\nNothing was modified.")
}

func missing(w io.Writer, title string, list []Missing, limit int) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s: %d\n", title, len(list))
	for i, m := range list {
		if i >= limit {
			fmt.Fprintf(w, "  ... and %d more\n", len(list)-limit)
			break
		}
		fmt.Fprintf(w, "  %-64s %4d refs  (%s)\n", Truncate(m.Path, 64), m.Refs, m.Where)
	}
}

func sortDirs(d []Directory) {
	sort.Slice(d, func(i, j int) bool { return d[i].Bytes > d[j].Bytes })
}

// Truncate keeps the tail, because the distinguishing part of a Foundry path
// is at the end and the shared prefix carries no information.
func Truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return "..." + string(r[len(r)-n+3:])
}

func Bytes(b int64) string {
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
