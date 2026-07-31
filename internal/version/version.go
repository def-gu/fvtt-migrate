package version

import "strings"

type Verdict string

const (
	OK           Verdict = "ok"
	Untested     Verdict = "untested"
	Incompatible Verdict = "incompatible"
	Unknown      Verdict = "unknown"
)

// Parse reads a Foundry version into numeric components. It tolerates a "v"
// prefix, trailing pre-release text and any number of components, none of which
// the manifest schema forbids and all of which occur in practice.
func Parse(s string) []int {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if s == "" {
		return nil
	}

	var out []int
	for _, part := range strings.Split(s, ".") {
		n, digits := 0, 0
		for _, r := range part {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
			digits++
		}
		if digits == 0 && len(out) > 0 {
			break
		}
		out = append(out, n)
	}
	return out
}

func Compare(a, b string) int {
	return compare(Parse(a), Parse(b))
}

// CompareBound compares a version against a declared bound at the bound's own
// precision. Manifests write `"verified": "13"` to mean the whole v13
// generation, so 13.351 must read as equal to it rather than greater.
func CompareBound(v, bound string) int {
	pv, pb := Parse(v), Parse(bound)
	if len(pb) == 0 {
		return 0
	}
	if len(pv) > len(pb) {
		pv = pv[:len(pb)]
	}
	return compare(pv, pb)
}

func compare(a, b []int) int {
	n := max(len(a), len(b))
	for i := 0; i < n; i++ {
		x, y := at(a, i), at(b, i)
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func at(s []int, i int) int {
	if i < len(s) {
		return s[i]
	}
	return 0
}

// Check reports how a package declaring these bounds relates to a core version.
func Check(minimum, verified, maximum, core string) Verdict {
	if core == "" {
		return Unknown
	}
	if minimum != "" && CompareBound(core, minimum) < 0 {
		return Incompatible
	}
	if maximum != "" && CompareBound(core, maximum) > 0 {
		return Incompatible
	}
	if verified != "" && CompareBound(core, verified) > 0 {
		return Untested
	}
	if minimum == "" && verified == "" && maximum == "" {
		return Unknown
	}
	return OK
}
