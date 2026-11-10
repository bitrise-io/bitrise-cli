package rde

import (
	"encoding/json"
	"strings"
	"testing"
)

// FilterOS keeps the rows, sessions and machine families of one OS, leaves
// every totals object as it was, and never turns a list into null.
func TestWorkspaceUsage_FilterOS(t *testing.T) {
	linux := PlatformUsage{SessionCount: 1, VCPU: 8, MemoryGB: 32}
	macos := PlatformUsage{SessionCount: 1, VCPU: 4, MemoryGB: 6}
	unknown := PlatformUsage{SessionCount: 1}
	report := WorkspaceUsage{
		Totals: UsageTotals{Linux: PlatformUsage{SessionCount: 2, VCPU: 16, MemoryGB: 64}, Macos: macos, Unknown: unknown},
		Users: []UserUsage{
			{
				Email:  "alice@example.com",
				Totals: UsageTotals{Linux: linux, Macos: macos},
				Sessions: []UsageSession{
					{ID: "s-linux", OS: UsageOSLinux},
					{ID: "s-macos", OS: UsageOSMacos},
				},
			},
			// On Linux only, and on a workspace where the session list is
			// not enabled the row still counts through its totals.
			{Email: "bob@example.com", Totals: UsageTotals{Linux: linux}, Sessions: []UsageSession{}},
			// Unknown OS only: on neither OS.
			{IsWorkspace: true, Totals: UsageTotals{Unknown: unknown}, Sessions: []UsageSession{{ID: "s-unknown"}}},
		},
		UnknownMachineTypeCount: 1,
		ComputeQuotas: []ComputeQuota{
			{Family: "linux-lc01", OS: UsageOSLinux},
			{Family: "m2pro", OS: UsageOSMacos},
			{Family: "mystery"},
		},
	}

	got := report.FilterOS(UsageOSMacos)
	if len(got.Users) != 1 || got.Users[0].Email != "alice@example.com" {
		t.Fatalf("macos users = %+v, want alice only", got.Users)
	}
	if s := got.Users[0].Sessions; len(s) != 1 || s[0].ID != "s-macos" {
		t.Errorf("macos sessions = %+v, want s-macos only", s)
	}
	if got.Users[0].Totals != report.Users[0].Totals || got.Totals != report.Totals {
		t.Errorf("totals changed: %+v / %+v", got.Totals, got.Users[0].Totals)
	}
	if len(got.ComputeQuotas) != 1 || got.ComputeQuotas[0].Family != "m2pro" {
		t.Errorf("macos quotas = %+v, want m2pro only", got.ComputeQuotas)
	}
	if got.UnknownMachineTypeCount != 1 {
		t.Errorf("unknown_machine_type_count = %d, want 1", got.UnknownMachineTypeCount)
	}

	got = report.FilterOS(UsageOSLinux)
	if len(got.Users) != 2 || got.Users[0].Email != "alice@example.com" || got.Users[1].Email != "bob@example.com" {
		t.Fatalf("linux users = %+v, want alice and bob", got.Users)
	}
	if s := got.Users[0].Sessions; len(s) != 1 || s[0].ID != "s-linux" {
		t.Errorf("linux sessions = %+v, want s-linux only", s)
	}

	// The receiver is untouched.
	if len(report.Users) != 3 || len(report.Users[0].Sessions) != 2 || len(report.ComputeQuotas) != 3 {
		t.Errorf("FilterOS changed its receiver: %+v", report)
	}

	// Nothing on the OS: empty arrays in JSON, never null.
	empty := WorkspaceUsage{Users: []UserUsage{{Totals: UsageTotals{Linux: linux}}}}.FilterOS(UsageOSMacos)
	out, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"users":[]`, `"compute_quotas":[]`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("JSON missing %s:\n%s", want, out)
		}
	}
}
