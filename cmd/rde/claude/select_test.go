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

func TestReuseSummary(t *testing.T) {
	if got := reuseSummary("osx-26-edge", "g2.mac.m2pro.4c-6g"); got != "osx-26-edge · g2.mac.m2pro.4c-6g · 4 vCPU · 6 GB" {
		t.Errorf("reuseSummary with specs = %q", got)
	}
	// A machine name without a parseable spec tail omits the hint.
	if got := reuseSummary("linux-bitvirt-2026", "g2.mac"); got != "linux-bitvirt-2026 · g2.mac" {
		t.Errorf("reuseSummary without specs = %q", got)
	}
}

func TestBuildReuseMenu(t *testing.T) {
	const summary = "img · mt"
	for _, tc := range []struct {
		name         string
		multiImage   bool
		multiMachine bool
		wantTitles   []string
		wantActions  []reuseAction
	}{
		{"both changeable", true, true,
			[]string{"Use this setup", "Change image", "Change machine type"},
			[]reuseAction{reuseUse, reuseChangeImage, reuseChangeMachine}},
		{"single machine type hides machine row", true, false,
			[]string{"Use this setup", "Change image"},
			[]reuseAction{reuseUse, reuseChangeImage}},
		{"single image hides image row", false, true,
			[]string{"Use this setup", "Change machine type"},
			[]reuseAction{reuseUse, reuseChangeMachine}},
		{"single of both → reuse only", false, false,
			[]string{"Use this setup"},
			[]reuseAction{reuseUse}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items, actions := buildReuseMenu(tc.multiImage, tc.multiMachine, summary)
			if len(items) != len(tc.wantTitles) {
				t.Fatalf("items = %d, want %d", len(items), len(tc.wantTitles))
			}
			for i, want := range tc.wantTitles {
				if items[i].Title != want {
					t.Errorf("items[%d].Title = %q, want %q", i, items[i].Title, want)
				}
			}
			if items[0].Desc != summary {
				t.Errorf("reuse row Desc = %q, want %q", items[0].Desc, summary)
			}
			for i, want := range tc.wantActions {
				if actions[i] != want {
					t.Errorf("actions[%d] = %d, want %d", i, actions[i], want)
				}
			}
		})
	}
}

func TestMachineSpecHint(t *testing.T) {
	for _, tc := range []struct {
		name string
		want string
	}{
		{"g2.mac.m2pro.4c-6g", "4 vCPU · 6 GB"},
		{"g2.linux.amd-zen5.8c-32g", "8 vCPU · 32 GB"},
		{"8c-16g", "8 vCPU · 16 GB"}, // bare segment, no dots
		{"g2.mac", ""},               // last segment "mac" doesn't match
		{"g2.linux.bad", ""},
		{"", ""},
	} {
		if got := machineSpecHint(tc.name); got != tc.want {
			t.Errorf("machineSpecHint(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
