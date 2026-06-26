package update

import "testing"

func TestParseRelease(t *testing.T) {
	cases := []struct {
		in   string
		want semver
		ok   bool
	}{
		{"1.2.3", semver{1, 2, 3}, true},
		{"v1.2.3", semver{1, 2, 3}, true},
		{"  v0.0.1 ", semver{0, 0, 1}, true},
		{"10.20.30", semver{10, 20, 30}, true},
		{"dev", semver{}, false},
		{"1.2", semver{}, false},
		{"1.2.3.4", semver{}, false},
		{"1.2.3-rc1", semver{}, false},
		{"v1.2.3-5-gabc123", semver{}, false}, // git-describe dev build
		{"1.2.x", semver{}, false},
		{"", semver{}, false},
		{"v", semver{}, false},
		{"-1.2.3", semver{}, false},
	}
	for _, c := range cases {
		got, ok := parseRelease(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseRelease(%q) = (%+v, %v), want (%+v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"1.0.0", "1.0.1", true},
		{"1.0.0", "1.1.0", true},
		{"1.0.0", "2.0.0", true},
		{"1.9.9", "2.0.0", true},
		{"v1.2.0", "v1.3.0", true},
		{"1.0.0", "1.0.0", false}, // equal
		{"2.0.0", "1.0.0", false}, // ahead
		{"1.2.0", "1.0.9", false},
		{"dev", "2.0.0", false},     // unparseable current → never behind
		{"1.0.0", "garbage", false}, // unparseable latest
		{"1.0.0-5-gabc", "1.0.0", false},
	}
	for _, c := range cases {
		if got := isNewer(c.current, c.latest); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestIsRelease(t *testing.T) {
	for _, in := range []string{"1.2.3", "v1.2.3", "0.0.1"} {
		if !IsRelease(in) {
			t.Errorf("IsRelease(%q) = false, want true", in)
		}
	}
	for _, in := range []string{"dev", "", "1.2", "v1.2.3-5-gabc123", "1.2.3-next"} {
		if IsRelease(in) {
			t.Errorf("IsRelease(%q) = true, want false", in)
		}
	}
}

func TestDisplayVersion(t *testing.T) {
	cases := map[string]string{
		"v1.2.3":   "1.2.3",
		"1.2.3":    "1.2.3",
		" v2.0.0 ": "2.0.0",
	}
	for in, want := range cases {
		if got := displayVersion(in); got != want {
			t.Errorf("displayVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
