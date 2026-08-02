package report

import (
	"strings"
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
)

func TestTargetVersionsOfferEveryGenerationSorted(t *testing.T) {
	inv := &foundry.Inventory{
		Worlds: []foundry.Package{
			{CoreVersion: "13.348"},
			{CoreVersion: "13.351"},
			{CoreVersion: "12.331"},
		},
		Modules: []foundry.Package{
			{Compat: foundry.Compatibility{Minimum: "13", Verified: "14.364"}},
			{Compat: foundry.Compatibility{Verified: "14"}},
			{Compat: foundry.Compatibility{Verified: "", Maximum: "13.999"}},
			{Compat: foundry.Compatibility{Minimum: "0.6.5", Verified: "11.315"}},
		},
	}

	got := TargetVersions(inv)
	want := "14.364,13.351"
	if strings.Join(got, ",") != want {
		t.Errorf("versions = %v, want %s", got, want)
	}
}

func TestTargetVersionsOfNothing(t *testing.T) {
	got := TargetVersions(&foundry.Inventory{})
	if got == nil || len(got) != 0 {
		t.Errorf("versions = %#v, want an empty list rather than null", got)
	}
}
