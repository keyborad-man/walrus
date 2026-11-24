package version

import "testing"

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b   string
		want   int
		wantOK bool
	}{
		{"v0.1.0", "v0.2.0", -1, true},
		{"0.2.0", "v0.2.0", 0, true},
		{"v1.0.0", "v0.9.9", 1, true},
		{"v1.2.3-rc.1", "v1.2.3", -1, true},
		{"v1.2.3", "v1.2.3-rc.1", 1, true},
		{"v1.2.3+build.5", "v1.2.3", 0, true},
		{"v1.2", "v1.2.0", 0, true},
		{"dev", "v0.1.0", 0, false},
		{"main-abc1234", "v0.1.0", 0, false},
	}
	for _, tc := range cases {
		got, ok := CompareSemver(tc.a, tc.b)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("CompareSemver(%q, %q) = (%d, %v), want (%d, %v)", tc.a, tc.b, got, ok, tc.want, tc.wantOK)
		}
	}
}
