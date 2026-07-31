package version

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want []int
	}{
		{"1.3.0", []int{1, 3, 0}},
		{"1.2", []int{1, 2}},
		{"v1.2.0", []int{1, 2, 0}},
		{"1.13.5.1", []int{1, 13, 5, 1}},
		{"13.351", []int{13, 351}},
		{"8.10", []int{8, 10}},
		{"2.1.0-beta3", []int{2, 1, 0}},
		{"", nil},
	}
	for _, c := range cases {
		got := Parse(c.in)
		if len(got) != len(c.want) {
			t.Errorf("Parse(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("Parse(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.3.0", "1.3.0", 0},
		{"1.3.0", "1.3", 0},
		{"8.10", "8.9", 1},
		{"8.9", "8.10", -1},
		{"1.13.5.1", "1.13.5", 1},
		{"v1.2.0", "1.2.0", 0},
		{"13.351", "14.363", -1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCompareBoundUsesBoundPrecision(t *testing.T) {
	cases := []struct {
		v, bound string
		want     int
	}{
		{"13.351", "13", 0},
		{"14.363", "13", 1},
		{"12.331", "13", -1},
		{"13.351", "13.351", 0},
		{"13.351", "13.400", -1},
	}
	for _, c := range cases {
		if got := CompareBound(c.v, c.bound); got != c.want {
			t.Errorf("CompareBound(%q, %q) = %d, want %d", c.v, c.bound, got, c.want)
		}
	}
}

func TestCheck(t *testing.T) {
	cases := []struct {
		name                       string
		minimum, verified, maximum string
		core                       string
		want                       Verdict
	}{
		{"verified generation matches", "0.6.5", "14", "", "14.363", OK},
		{"below minimum", "13", "14", "", "12.331", Incompatible},
		{"above maximum", "11", "12", "12", "13.351", Incompatible},
		{"past verified, no maximum", "10", "12", "", "13.351", Untested},
		{"within verified", "10", "13", "", "13.351", OK},
		{"nothing declared", "", "", "", "13.351", Unknown},
		{"core unknown", "10", "13", "", "", Unknown},
	}
	for _, c := range cases {
		if got := Check(c.minimum, c.verified, c.maximum, c.core); got != c.want {
			t.Errorf("%s: Check() = %q, want %q", c.name, got, c.want)
		}
	}
}
