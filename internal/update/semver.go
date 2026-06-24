package update

import (
	"strconv"
	"strings"
)

// semver is a parsed MAJOR.MINOR.PATCH version. Pre-release and build metadata
// are intentionally not modelled: the update notice only ever compares clean
// release tags (see parseRelease).
type semver struct {
	major, minor, patch int
}

// parseRelease parses a clean release version: an optional leading "v" then
// MAJOR.MINOR.PATCH with no pre-release or build suffix. Anything else — "dev",
// a `git describe` build like "v1.2.3-5-gabc123", a "-next" snapshot, or a
// malformed string — returns ok=false, which callers treat as "not a
// comparable release" and stay silent. This is what keeps the notice from
// firing on local dev builds that sit ahead of the last tag.
func parseRelease(s string) (semver, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	out := [3]int{}
	for i, p := range parts {
		// Reject leading "+"/"-" and any non-digit run (e.g. "3-next") so only
		// pure release cores parse.
		if p == "" || !isDigits(p) {
			return semver{}, false
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return semver{}, false
		}
		out[i] = n
	}
	return semver{major: out[0], minor: out[1], patch: out[2]}, true
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// less reports whether a sorts before b.
func (a semver) less(b semver) bool {
	switch {
	case a.major != b.major:
		return a.major < b.major
	case a.minor != b.minor:
		return a.minor < b.minor
	default:
		return a.patch < b.patch
	}
}

// IsRelease reports whether s is a clean release version this package can
// compare. The cmd layer uses it to skip the network check entirely for dev
// builds, so they never touch GitHub or write a cache file.
func IsRelease(s string) bool {
	_, ok := parseRelease(s)
	return ok
}

// isNewer reports whether latest is a strictly higher release than current.
// If either side is not a clean release, it returns false — an unparseable
// version is never "behind".
func isNewer(current, latest string) bool {
	cur, ok1 := parseRelease(current)
	lat, ok2 := parseRelease(latest)
	if !ok1 || !ok2 {
		return false
	}
	return cur.less(lat)
}

// displayVersion strips a leading "v" so notices read "1.2.0 → 1.3.0"
// regardless of whether the tag carried the prefix.
func displayVersion(s string) string {
	return strings.TrimPrefix(strings.TrimSpace(s), "v")
}
