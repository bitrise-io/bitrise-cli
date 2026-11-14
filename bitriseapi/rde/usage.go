package rde

import (
	"context"
	"fmt"
)

// PlatformUsage aggregates active sessions on one OS platform. The backend
// omits zero-valued fields from the JSON, so absent means 0.
type PlatformUsage struct {
	SessionCount int32 `json:"sessionCount"`
	VCPU         int32 `json:"vcpu"`
	MemoryGB     int32 `json:"memoryGb"`
}

// UsageTotals splits active-session usage by OS platform. Buckets are
// pointers because the backend may omit an empty bucket object entirely.
type UsageTotals struct {
	Linux   *PlatformUsage `json:"linux"`
	Macos   *PlatformUsage `json:"macos"`
	Unknown *PlatformUsage `json:"unknown"`
}

// UserUsage is one row of the per-user usage breakdown. The workspace-owned
// bucket row (IsWorkspace) carries no user identity.
type UserUsage struct {
	UserID      string       `json:"userId"`
	UserSlug    string       `json:"userSlug"`
	Email       string       `json:"email"`
	Username    string       `json:"username"`
	IsWorkspace bool         `json:"isWorkspace"`
	Totals      *UsageTotals `json:"totals"`
	// Sessions are the active sessions behind this row's totals, oldest
	// first. The backend fills it only for a workspace where terminating
	// any session is enabled; absent otherwise.
	Sessions []UsageSession `json:"sessions"`
}

// UsageSession is one active session in the usage report: enough to decide
// whether to terminate it (TerminateSession with its ID). The backend omits
// zero-valued fields, so absent means 0 / empty.
type UsageSession struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// OS is "linux" or "macos"; empty when the backend could not tell.
	OS          string `json:"os"`
	MachineType string `json:"machineType"`
	// VCPU and MemoryGB are 0 when the machine type's spec is unknown.
	VCPU     int32 `json:"vcpu"`
	MemoryGB int32 `json:"memoryGb"`
	// Status is the raw SESSION_STATUS_* enum string (starting, running,
	// terminating or draining).
	Status string `json:"status"`
	// CreatedAt is RFC3339. A session terminated and restored since keeps
	// its original creation time.
	CreatedAt string `json:"createdAt"`
	// HeldByWarmPoolID is the ID of the warm pool holding the session:
	// unclaimed inventory, or the machine a pool keeps after its setup
	// failed. Such a session cannot be terminated directly. Empty for every
	// other session, including ones already claimed from a pool.
	HeldByWarmPoolID string `json:"heldByWarmPoolId"`
}

// ComputeQuota is the workspace's vCPU quota on one machine family and the
// vCPUs its machines hold right now. The backend omits zero-valued fields,
// so absent means 0.
type ComputeQuota struct {
	Family      string `json:"family"`
	DisplayName string `json:"displayName"`
	UsedVCPU    int32  `json:"usedVcpu"`
	QuotaVCPU   int32  `json:"quotaVcpu"`
	// OS is "linux" or "macos"; empty when the backend could not tell.
	OS string `json:"os"`
}

// WorkspaceUsage is the workspace usage report: a point-in-time snapshot of
// the sessions currently consuming resources, split by OS and by user, plus
// the workspace's vCPU quota per machine family.
type WorkspaceUsage struct {
	Totals *UsageTotals `json:"totals"`
	Users  []UserUsage  `json:"users"`
	// UnknownMachineTypeCount is the number of active sessions whose machine
	// type had no resolvable vCPU/RAM spec; they contribute 0 to the sums, so
	// totals may undercount when this is non-zero.
	UnknownMachineTypeCount int32 `json:"unknownMachineTypeCount"`
	// ComputeQuotas is the vCPU quota per machine family. Empty when the
	// backend could not read the quota.
	ComputeQuotas []ComputeQuota `json:"computeQuotas"`
}

// GetWorkspaceUsage returns the workspace's active-session usage snapshot.
// Requires the workspace's billing-view permission (workspace owners and
// billing-managing custom roles); other members get a permission error. A
// Workspace API Token with Remote Dev Environments access and a workspace
// role that can manage the workspace may call it too.
// Endpoint: GET /v1/workspaces/{workspaceId}/usage.
func (c *Client) GetWorkspaceUsage(ctx context.Context, workspaceID string) (WorkspaceUsage, error) {
	if workspaceID == "" {
		return WorkspaceUsage{}, fmt.Errorf("workspace ID is required")
	}
	var resp WorkspaceUsage
	if err := c.getJSON(ctx, wsPath(workspaceID, "/usage"), &resp); err != nil {
		return WorkspaceUsage{}, err
	}
	return resp, nil
}
