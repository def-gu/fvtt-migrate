package plan

import (
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/version"
)

func TestRecommend(t *testing.T) {
	cases := []struct {
		name                 string
		installed, available version.Verdict
		haveUpdate           bool
		want                 Recommendation
	}{
		{"nothing newer, runs fine", version.OK, "", false, NoUpdate},
		{"nothing newer, does not run", version.Incompatible, "", false, Blocked},
		{"update drops the target", version.OK, version.Incompatible, true, Keep},
		{"both run", version.OK, version.OK, true, Upgrade},
		{"installed broken, update fixes", version.Incompatible, version.OK, true, Required},
		{"neither runs", version.Incompatible, version.Incompatible, true, Blocked},
		{"untested counts as running", version.Untested, version.Incompatible, true, Keep},
	}

	for _, c := range cases {
		if got := Recommend(c.installed, c.available, c.haveUpdate); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestWidens(t *testing.T) {
	cases := []struct {
		name              string
		minimum, verified string
		target            string
		want              bool
	}{
		{"keeps 13, ready for 14", "13", "14", "13.351", true},
		{"same generation only", "13", "13", "13.351", false},
		{"drops 13", "14", "14", "13.351", false},
		{"old floor, ready for 14", "11", "14", "13.351", true},
		{"nothing declared", "", "", "13.351", false},
	}

	for _, c := range cases {
		if got := Widens(c.minimum, c.verified, c.target); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestRecommendationsFlipWithTarget(t *testing.T) {
	installed := func(core string) version.Verdict { return version.Check("13", "13", "13", core) }
	available := func(core string) version.Verdict { return version.Check("14", "14", "", core) }

	if got := Recommend(installed("13.351"), available("13.351"), true); got != Keep {
		t.Errorf("targeting 13: got %q, want keep", got)
	}
	if got := Recommend(installed("14.363"), available("14.363"), true); got != Required {
		t.Errorf("targeting 14: got %q, want required", got)
	}
}

func TestUntestedIsNotTreatedAsBroken(t *testing.T) {
	got := Recommend(
		version.Check("13", "13", "", "14.363"),
		version.Check("14", "14", "", "14.363"),
		true)
	if got != Upgrade {
		t.Errorf("got %q, want upgrade: stale `verified` is not evidence of breakage", got)
	}
}
