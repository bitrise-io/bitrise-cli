package rde

import (
	"context"
	"testing"
)

func TestGetWorkspaceUsage_PathAndParse(t *testing.T) {
	rs := newRecordingServer(t, `{
		"totals": {"linux": {"sessionCount": 2, "vcpu": 4, "memoryGb": 16}},
		"users": [{
			"userId": "u-1", "userSlug": "alice-slug", "email": "alice@example.com",
			"totals": {"linux": {"sessionCount": 2, "vcpu": 4, "memoryGb": 16}}
		}],
		"unknownMachineTypeCount": 1,
		"computeQuotas": [
			{"family": "m2pro", "displayName": "M2 Pro", "usedVcpu": 12, "quotaVcpu": 48, "os": "macos"},
			{"family": "amd-zen4", "displayName": "AMD Zen 4"}
		]
	}`)

	usage, err := rs.client().GetWorkspaceUsage(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("GetWorkspaceUsage: %v", err)
	}
	if want := "/v1/workspaces/ws-1/usage"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	if usage.Totals == nil || usage.Totals.Linux == nil || usage.Totals.Linux.VCPU != 4 || usage.Totals.Linux.MemoryGB != 16 {
		t.Errorf("totals = %+v", usage.Totals)
	}
	// The backend omits empty bucket objects; absent buckets stay nil here
	// (the service layer densifies).
	if usage.Totals.Macos != nil {
		t.Errorf("macos bucket = %+v, want nil for an omitted bucket", usage.Totals.Macos)
	}
	if len(usage.Users) != 1 || usage.Users[0].UserSlug != "alice-slug" || usage.Users[0].UserID != "u-1" {
		t.Errorf("users = %+v", usage.Users)
	}
	if usage.UnknownMachineTypeCount != 1 {
		t.Errorf("unknownMachineTypeCount = %d, want 1", usage.UnknownMachineTypeCount)
	}
	wantQuotas := []ComputeQuota{
		{Family: "m2pro", DisplayName: "M2 Pro", UsedVCPU: 12, QuotaVCPU: 48, OS: "macos"},
		{Family: "amd-zen4", DisplayName: "AMD Zen 4"},
	}
	if len(usage.ComputeQuotas) != len(wantQuotas) {
		t.Fatalf("computeQuotas = %+v, want %+v", usage.ComputeQuotas, wantQuotas)
	}
	for i, want := range wantQuotas {
		if usage.ComputeQuotas[i] != want {
			t.Errorf("computeQuotas[%d] = %+v, want %+v", i, usage.ComputeQuotas[i], want)
		}
	}
}

// The per-user session list arrives in the wire's lowerCamelCase, with
// zero-valued fields omitted.
func TestGetWorkspaceUsage_ParsesUserSessions(t *testing.T) {
	rs := newRecordingServer(t, `{
		"users": [
			{
				"userSlug": "alice-slug", "email": "alice@example.com",
				"sessions": [{
					"id": "11111111-1111-1111-1111-111111111111", "name": "api-work", "os": "linux",
					"machineType": "g2.linux.large", "vcpu": 8, "memoryGb": 32,
					"status": "SESSION_STATUS_RUNNING", "createdAt": "2026-09-01T08:00:00Z"
				}]
			},
			{
				"isWorkspace": true,
				"sessions": [{
					"id": "22222222-2222-2222-2222-222222222222", "name": "warm-1",
					"status": "SESSION_STATUS_STARTING", "createdAt": "2026-09-02T08:00:00Z",
					"heldByWarmPoolId": "33333333-3333-3333-3333-333333333333"
				}]
			},
			{"userSlug": "bob-slug"}
		]
	}`)

	usage, err := rs.client().GetWorkspaceUsage(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("GetWorkspaceUsage: %v", err)
	}
	if len(usage.Users) != 3 {
		t.Fatalf("users = %+v, want 3 rows", usage.Users)
	}
	want := UsageSession{
		ID: "11111111-1111-1111-1111-111111111111", Name: "api-work", OS: "linux",
		MachineType: "g2.linux.large", VCPU: 8, MemoryGB: 32,
		Status: "SESSION_STATUS_RUNNING", CreatedAt: "2026-09-01T08:00:00Z",
	}
	if got := usage.Users[0].Sessions; len(got) != 1 || got[0] != want {
		t.Errorf("users[0].sessions = %+v, want [%+v]", got, want)
	}
	wantHeld := UsageSession{
		ID: "22222222-2222-2222-2222-222222222222", Name: "warm-1",
		Status: "SESSION_STATUS_STARTING", CreatedAt: "2026-09-02T08:00:00Z",
		HeldByWarmPoolID: "33333333-3333-3333-3333-333333333333",
	}
	if got := usage.Users[1].Sessions; len(got) != 1 || got[0] != wantHeld {
		t.Errorf("users[1].sessions = %+v, want [%+v]", got, wantHeld)
	}
	// A workspace where the list is not enabled omits it.
	if got := usage.Users[2].Sessions; len(got) != 0 {
		t.Errorf("users[2].sessions = %+v, want none", got)
	}
}

func TestGetWorkspaceUsage_ValidationGuard(t *testing.T) {
	rs := newRecordingServer(t, `{}`)

	if _, err := rs.client().GetWorkspaceUsage(context.Background(), ""); err == nil {
		t.Error("expected validation error, got nil")
	}
	if rs.hits != 0 {
		t.Errorf("validation guard made %d HTTP call(s); should short-circuit", rs.hits)
	}
}
