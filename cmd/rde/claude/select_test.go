package claude

import (
	"strings"
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

func TestUniqueStacks(t *testing.T) {
	stacks := []internalrde.Stack{
		{ID: "linux", Title: "Ubuntu 24.04"},
		{ID: "linux", Title: "Ubuntu 24.04"}, // duplicate id
		{ID: "mac", Title: "Xcode 16.0", IsDefault: true},
		{ID: "win", Title: "Windows"},
	}
	ids, titleByID, backendDefault := uniqueStacks(stacks)
	want := []string{"linux", "mac", "win"}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("ids[%d] = %q, want %q", i, ids[i], want[i])
		}
	}
	if backendDefault != "mac" {
		t.Errorf("backendDefault = %q, want mac", backendDefault)
	}
	if titleByID["mac"] != "Xcode 16.0" {
		t.Errorf("titleByID[mac] = %q, want Xcode 16.0", titleByID["mac"])
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

// TestResolveDefault_StackSwitchFallsBackToFirst documents the machine-type
// behavior when the user switches stacks: the saved machine type (valid for the
// old stack) and the backend default both may be absent from the new stack's
// compatible list, so selection falls back to the first available.
func TestResolveDefault_StackSwitchFallsBackToFirst(t *testing.T) {
	// Compatible machine types for the newly-chosen stack.
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

func TestStackTitle(t *testing.T) {
	titleByID := map[string]string{
		"osx-xcode-16.0.x-edge": "Xcode 16.0",
		"linux-ubuntu-24.04":    "Ubuntu 24.04",
		"osx-no-title":          "", // backend supplied no title
	}
	for in, want := range map[string]string{
		"osx-xcode-16.0.x-edge": "Xcode 16.0",
		"linux-ubuntu-24.04":    "Ubuntu 24.04",
		"osx-no-title":          "osx-no-title",  // empty title → raw id
		"unknown-stack":         "unknown-stack", // absent → raw id
	} {
		if got := stackTitle(titleByID, in); got != want {
			t.Errorf("stackTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReuseDetail(t *testing.T) {
	// Two lines: the stack title, then the machine display name plus its spec in
	// parentheses.
	got := reuseDetail("Xcode 26", "M2 Pro Large", "6 vCPU · 14 GB")
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("reuseDetail lines = %d, want 2: %q", len(lines), got)
	}
	if !strings.Contains(lines[0], "Stack") || !strings.Contains(lines[0], "Xcode 26") {
		t.Errorf("stack line = %q", lines[0])
	}
	if !strings.Contains(lines[1], "Machine type") || !strings.Contains(lines[1], "M2 Pro Large") || !strings.Contains(lines[1], "6 vCPU · 14 GB") {
		t.Errorf("machine line = %q", lines[1])
	}
	// No spec → no parenthetical.
	if got := reuseDetail("Ubuntu 24.04", "g2.mac", ""); strings.Contains(got, "(") {
		t.Errorf("reuseDetail without specs should have no parens: %q", got)
	}
}

func TestBuildReuseMenu(t *testing.T) {
	for _, tc := range []struct {
		name         string
		multiStack   bool
		multiMachine bool
		wantTitles   []string
		wantActions  []reuseAction
	}{
		{"both changeable", true, true,
			[]string{"Use this setup", "Change stack", "Change machine type"},
			[]reuseAction{reuseUse, reuseChangeStack, reuseChangeMachine}},
		{"single machine type hides machine row", true, false,
			[]string{"Use this setup", "Change stack"},
			[]reuseAction{reuseUse, reuseChangeStack}},
		{"single stack hides stack row", false, true,
			[]string{"Use this setup", "Change machine type"},
			[]reuseAction{reuseUse, reuseChangeMachine}},
		{"single of both → reuse only", false, false,
			[]string{"Use this setup"},
			[]reuseAction{reuseUse}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items, actions := buildReuseMenu(tc.multiStack, tc.multiMachine)
			if len(items) != len(tc.wantTitles) {
				t.Fatalf("items = %d, want %d", len(items), len(tc.wantTitles))
			}
			for i, want := range tc.wantTitles {
				if items[i].Title != want {
					t.Errorf("items[%d].Title = %q, want %q", i, items[i].Title, want)
				}
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

func TestMachineItem_UsesBackendTitleAndSpecs(t *testing.T) {
	byName := indexMachineTypes([]internalrde.MachineType{
		{Name: "g2.mac.m2pro.4c-6g", Title: "M2 Pro Large", CPU: "4 vCPU", RAM: "6 GB"},
		{Name: "g2.bare"}, // no friendly metadata
	})
	item := machineItem(byName)

	// Friendly title becomes the row title; the specs and the contract name
	// fill the dim secondary text so the name stays discoverable.
	got := item("g2.mac.m2pro.4c-6g")
	if got.Title != "M2 Pro Large" {
		t.Errorf("title = %q, want M2 Pro Large", got.Title)
	}
	for _, want := range []string{"4 vCPU", "6 GB", "g2.mac.m2pro.4c-6g"} {
		if !strings.Contains(got.Desc, want) {
			t.Errorf("desc %q missing %q", got.Desc, want)
		}
	}

	// No backend metadata → row title is the raw name (no name duplicated in desc).
	bare := item("g2.bare")
	if bare.Title != "g2.bare" {
		t.Errorf("title = %q, want raw name g2.bare", bare.Title)
	}

	// Name absent from the catalog map → graceful fallback to the raw name.
	if unknown := item("nope"); unknown.Title != "nope" {
		t.Errorf("title = %q, want nope", unknown.Title)
	}
}

func TestMachineLabel(t *testing.T) {
	cases := map[string]struct {
		mt   internalrde.MachineType
		want string
	}{
		"backend title + specs": {
			internalrde.MachineType{Name: "g2.mac.m2pro.12c-28g", Title: "M2 Pro Large", CPU: "12 vCPU", RAM: "28 GB"},
			"M2 Pro Large (12 vCPU · 28 GB)",
		},
		"no title, specs parsed from name": {
			internalrde.MachineType{Name: "g2.mac.m2pro.12c-28g"},
			"g2.mac.m2pro.12c-28g (12 vCPU · 28 GB)",
		},
		"no title, no parseable specs": {
			internalrde.MachineType{Name: "g2.bare"},
			"g2.bare",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := machineLabel(tc.mt); got != tc.want {
				t.Errorf("machineLabel = %q, want %q", got, tc.want)
			}
		})
	}
}
