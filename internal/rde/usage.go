package rde

import (
	"context"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

// PlatformUsage aggregates active sessions on one OS platform. Always dense:
// fields the backend omitted are 0.
type PlatformUsage struct {
	SessionCount int32 `json:"session_count"`
	VCPU         int32 `json:"vcpu"`
	MemoryGB     int32 `json:"memory_gb"`
}

// UsageTotals splits active-session usage by OS platform. Unknown holds
// sessions whose OS could not be determined.
type UsageTotals struct {
	Linux   PlatformUsage `json:"linux"`
	Macos   PlatformUsage `json:"macos"`
	Unknown PlatformUsage `json:"unknown"`
}

// UserUsage is one row of the per-user usage breakdown. The single
// workspace-owned bucket row (IsWorkspace true) carries no user identity.
//
// UserID is the user's Bitrise ID (the wire's slug — the identifier other
// Bitrise surfaces use; the backend's internal user UUID is not exposed).
// Best-effort: may be empty, so detect the workspace bucket via IsWorkspace,
// never via an empty UserID.
type UserUsage struct {
	UserID      string      `json:"user_id,omitempty"`
	Email       string      `json:"email,omitempty"`
	Username    string      `json:"username,omitempty"`
	IsWorkspace bool        `json:"is_workspace,omitempty"`
	Totals      UsageTotals `json:"totals"`
}

// ComputeQuota is the workspace's vCPU quota on one machine family and the
// vCPUs its machines hold right now. Always dense: fields the backend omitted
// are 0.
//
// UsedVCPU counts every active machine in the family, pool sessions
// included. It is counted from machines rather than sessions, so it
// can briefly differ from the session totals. Only families the plan gives a
// quota are listed. OS is "linux" or "macos", empty when the backend could not
// tell. A new session cannot start on a family once
// UsedVCPU plus the new machine's vCPUs would exceed QuotaVCPU.
type ComputeQuota struct {
	Family      string `json:"family"`
	DisplayName string `json:"display_name"`
	UsedVCPU    int32  `json:"used_vcpu"`
	QuotaVCPU   int32  `json:"quota_vcpu"`
	OS          string `json:"os"`
}

// WorkspaceUsage is a point-in-time snapshot of the sessions currently
// consuming resources in the workspace, split by OS and by user, plus the
// workspace's vCPU quota per machine family. It is not a historical or
// billing-period report.
type WorkspaceUsage struct {
	Totals UsageTotals `json:"totals"`
	Users  []UserUsage `json:"users"`
	// UnknownMachineTypeCount counts active sessions whose machine type had
	// no resolvable vCPU/RAM spec; they contribute 0 to the sums, so totals
	// may undercount when this is non-zero.
	UnknownMachineTypeCount int32 `json:"unknown_machine_type_count"`
	// ComputeQuotas is the vCPU quota per machine family, sorted by display
	// name. Empty (never null) when the backend could not read the quota.
	ComputeQuotas []ComputeQuota `json:"compute_quotas"`
}

// GetWorkspaceUsage returns the workspace's active-session usage snapshot.
// Requires the workspace's billing-view permission (workspace owners and
// billing-managing custom roles).
func (s *Service) GetWorkspaceUsage(ctx context.Context, workspaceID string) (WorkspaceUsage, error) {
	if s.client == nil {
		return WorkspaceUsage{}, errClient()
	}
	wire, err := s.client.GetWorkspaceUsage(ctx, workspaceID)
	if err != nil {
		return WorkspaceUsage{}, err
	}
	return usageFromAPI(wire), nil
}

// usageFromAPI densifies the wire report: the backend omits zero-valued
// fields and empty bucket objects, and the stable shape promises every
// bucket present with explicit zeros.
func usageFromAPI(w rdeapi.WorkspaceUsage) WorkspaceUsage {
	out := WorkspaceUsage{
		Totals:                  usageTotalsFromAPI(w.Totals),
		Users:                   make([]UserUsage, 0, len(w.Users)),
		UnknownMachineTypeCount: w.UnknownMachineTypeCount,
		ComputeQuotas:           make([]ComputeQuota, 0, len(w.ComputeQuotas)),
	}
	for _, u := range w.Users {
		out.Users = append(out.Users, UserUsage{
			UserID:      u.UserSlug,
			Email:       u.Email,
			Username:    u.Username,
			IsWorkspace: u.IsWorkspace,
			Totals:      usageTotalsFromAPI(u.Totals),
		})
	}
	for _, q := range w.ComputeQuotas {
		name := q.DisplayName
		if name == "" {
			name = q.Family
		}
		out.ComputeQuotas = append(out.ComputeQuotas, ComputeQuota{
			Family:      q.Family,
			DisplayName: name,
			UsedVCPU:    q.UsedVCPU,
			QuotaVCPU:   q.QuotaVCPU,
			OS:          q.OS,
		})
	}
	return out
}

func usageTotalsFromAPI(w *rdeapi.UsageTotals) UsageTotals {
	if w == nil {
		return UsageTotals{}
	}
	return UsageTotals{
		Linux:   platformUsageFromAPI(w.Linux),
		Macos:   platformUsageFromAPI(w.Macos),
		Unknown: platformUsageFromAPI(w.Unknown),
	}
}

func platformUsageFromAPI(w *rdeapi.PlatformUsage) PlatformUsage {
	if w == nil {
		return PlatformUsage{}
	}
	return PlatformUsage{
		SessionCount: w.SessionCount,
		VCPU:         w.VCPU,
		MemoryGB:     w.MemoryGB,
	}
}
