package claude

import (
	"testing"

	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func TestResolveDefault(t *testing.T) {
	names := []string{"alpha", "beta", "gamma"}
	cases := map[string]struct {
		pref           string
		backendDefault string
		want           int
	}{
		"saved pref wins":                  {pref: "gamma", backendDefault: "beta", want: 2},
		"backend default when no pref":     {pref: "", backendDefault: "beta", want: 1},
		"stale pref falls to backend":      {pref: "deleted", backendDefault: "beta", want: 1},
		"stale pref and backend → first":   {pref: "deleted", backendDefault: "also-gone", want: 0},
		"nothing set → first":              {pref: "", backendDefault: "", want: 0},
		"pref present, no backend default": {pref: "beta", backendDefault: "", want: 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := resolveDefault(names, tc.pref, tc.backendDefault); got != tc.want {
				t.Errorf("resolveDefault(%q, %q) = %d, want %d", tc.pref, tc.backendDefault, got, tc.want)
			}
		})
	}
}

func TestUniqueImageNames(t *testing.T) {
	images := []internalrde.Image{
		{Name: "linux", ClusterName: "a"},
		{Name: "linux", ClusterName: "b"}, // duplicate name across clusters
		{Name: "mac", ClusterName: "c", IsDefault: true},
		{Name: "win", ClusterName: "d"},
	}
	names, backendDefault := uniqueImageNames(images)
	want := []string{"linux", "mac", "win"}
	if len(names) != len(want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
	if backendDefault != "mac" {
		t.Errorf("backendDefault = %q, want mac", backendDefault)
	}
}

func TestUniqueMachineTypeNames_NoDefault(t *testing.T) {
	mts := []internalrde.MachineType{
		{Name: "small", ClusterName: "a"},
		{Name: "big", ClusterName: "b"},
	}
	names, backendDefault := uniqueMachineTypeNames(mts)
	if len(names) != 2 || names[0] != "small" || names[1] != "big" {
		t.Errorf("names = %v, want [small big]", names)
	}
	if backendDefault != "" {
		t.Errorf("backendDefault = %q, want empty", backendDefault)
	}
}

// TestResolveDefault_ImageSwitchFallsBackToFirst documents the machine-type
// behavior when the user switches images: the saved machine type (valid for the
// old image) and the backend default both may be absent from the new image's
// compatible list, so selection falls back to the first available.
func TestResolveDefault_ImageSwitchFallsBackToFirst(t *testing.T) {
	// Compatible machine types for the newly-chosen image.
	compatible := []string{"arm-small", "arm-big"}
	// Saved pref + backend default are both x86 types, not in the list.
	got := resolveDefault(compatible, "x86-small", "x86-default")
	if got != 0 {
		t.Errorf("got index %d, want 0 (first available)", got)
	}
}

func TestMoveToFront(t *testing.T) {
	cases := map[string]struct {
		in   []string
		idx  int
		want []string
	}{
		"middle moves to front, rest order kept": {in: []string{"a", "b", "c", "d"}, idx: 2, want: []string{"c", "a", "b", "d"}},
		"last moves to front":                    {in: []string{"a", "b", "c"}, idx: 2, want: []string{"c", "a", "b"}},
		"already first is unchanged":             {in: []string{"a", "b", "c"}, idx: 0, want: []string{"a", "b", "c"}},
		"out of range is unchanged":              {in: []string{"a", "b"}, idx: 5, want: []string{"a", "b"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := moveToFront(tc.in, tc.idx)
			if len(got) != len(tc.want) {
				t.Fatalf("moveToFront(%v, %d) = %v, want %v", tc.in, tc.idx, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("moveToFront(%v, %d) = %v, want %v", tc.in, tc.idx, got, tc.want)
				}
			}
		})
	}
}

func TestIndexOf(t *testing.T) {
	names := []string{"a", "b", "c"}
	if indexOf(names, "b") != 1 {
		t.Error("indexOf should find b at 1")
	}
	if indexOf(names, "") != -1 {
		t.Error("empty target should never match")
	}
	if indexOf(names, "z") != -1 {
		t.Error("absent target should be -1")
	}
}
