package plan

import (
	"strconv"

	"github.com/def-gu/fvtt-migrate/internal/version"
)

type Recommendation string

const (
	NoUpdate Recommendation = "none"
	Keep     Recommendation = "keep"
	Upgrade  Recommendation = "upgrade"
	Required Recommendation = "required"
	Blocked  Recommendation = "blocked"
)

// Authors routinely leave `verified` stale, so a version past it is not
// evidence of breakage and must still count as usable.
func runsOn(v version.Verdict) bool {
	return v == version.OK || v == version.Untested || v == version.Unknown
}

func Recommend(installed, available version.Verdict, haveUpdate bool) Recommendation {
	installedRuns := runsOn(installed)
	if !haveUpdate {
		if installedRuns {
			return NoUpdate
		}
		return Blocked
	}

	availableRuns := runsOn(available)
	switch {
	case installedRuns && !availableRuns:
		return Keep
	case !installedRuns && availableRuns:
		return Required
	case !installedRuns && !availableRuns:
		return Blocked
	default:
		return Upgrade
	}
}

func Widens(availableMinimum, availableVerified, target string) bool {
	if availableVerified == "" || target == "" {
		return false
	}
	if availableMinimum != "" && version.CompareBound(target, availableMinimum) < 0 {
		return false
	}
	return version.CompareBound(availableVerified, generation(target)) > 0
}

func generation(v string) string {
	parts := version.Parse(v)
	if len(parts) == 0 {
		return v
	}
	return strconv.Itoa(parts[0])
}
