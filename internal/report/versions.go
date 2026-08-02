package report

import (
	"sort"
	"strings"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/version"
)

// The offered versions are the generation the worlds are on plus every newer
// one the installed packages have been tested against, so moving to the next
// generation is a choice rather than something the list forbids.
//
// Only the verified bound is read. A maximum is routinely written as a sentinel
// such as 13.999, which names no build anyone can install. Older generations are
// left out because a world does not move backwards.
func TargetVersions(inv *foundry.Inventory) []string {
	best := map[int]string{}
	consider := func(v string, floor int) {
		parts := version.Parse(v)
		if len(parts) == 0 || parts[0] < floor {
			return
		}
		if held, ok := best[parts[0]]; !ok || version.Compare(v, held) > 0 {
			best[parts[0]] = strings.TrimSpace(v)
		}
	}

	newest := 0
	for _, w := range inv.Worlds {
		if parts := version.Parse(w.CoreVersion); len(parts) > 0 && parts[0] > newest {
			newest = parts[0]
		}
	}

	for _, w := range inv.Worlds {
		consider(w.CoreVersion, newest)
	}
	for _, p := range append(append([]foundry.Package{}, inv.Systems...), inv.Modules...) {
		consider(p.Compat.Verified, newest+1)
	}

	majors := make([]int, 0, len(best))
	for major := range best {
		majors = append(majors, major)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(majors)))

	out := []string{}
	for _, major := range majors {
		out = append(out, best[major])
	}
	return out
}
