package rde

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// WarmPool is the wire-format warm pool record. A pool is a stored
// session-create request — template, session input values, feature flags,
// machine and device overrides — plus an owner and a desired count. The
// backend keeps DesiredCount sessions of that configuration booted and idle
// so a session created with CreateSessionRequest.WarmPoolID is handed one of
// them instead of a cold start.
type WarmPool struct {
	ID           string `json:"id"`
	WorkspaceID  string `json:"workspaceId,omitempty"`
	Name         string `json:"name"`
	TemplateID   string `json:"templateId,omitempty"`
	TemplateName string `json:"templateName,omitempty"`
	// OwnerType is "user" (private to its creator) or "workspace" (shared
	// with every member); OwnerID is the user ID or workspace slug
	// accordingly. Same vocabulary as Session.OwnerType / OwnerID.
	OwnerType      string `json:"ownerType,omitempty"`
	OwnerID        string `json:"ownerId,omitempty"`
	CreatedByEmail string `json:"createdByEmail,omitempty"`
	// DesiredCount is how many warm sessions the backend keeps booted; 0
	// drains the pool but keeps it usable as a configuration preset.
	DesiredCount int `json:"desiredCount,omitempty"`
	// SessionInputs are the values the warm sessions are created with.
	// Secret values come back redacted (empty Value, IsSecret true) —
	// the internal/rde mapper masks them again as defense-in-depth.
	SessionInputs           []SessionInputValue `json:"sessionInputs,omitempty"`
	EnabledFeatureFlagNames []string            `json:"enabledFeatureFlagNames,omitempty"`
	// StackID, MachineType and Cluster override the template's; empty
	// means the template's value (cluster: resolved from stack + machine).
	StackID     string `json:"stackId,omitempty"`
	MachineType string `json:"machineType,omitempty"`
	Cluster     string `json:"cluster,omitempty"`
	// DeviceSpec overrides the template's declared device; nil boots the
	// template's device as declared (or none). NoDevice skips it.
	DeviceSpec *DeviceSpec     `json:"deviceSpec,omitempty"`
	NoDevice   bool            `json:"noDevice,omitempty"`
	Status     *WarmPoolStatus `json:"status,omitempty"`
	CreatedAt  string          `json:"createdAt,omitempty"`
	UpdatedAt  string          `json:"updatedAt,omitempty"`
}

// WarmPoolStatus is the live state of a warm pool.
type WarmPoolStatus struct {
	// Ready and Warming count the pool's current inventory.
	Ready   int `json:"ready,omitempty"`
	Warming int `json:"warming,omitempty"`
	// ClaimedTotal and ColdTotal are lifetime counters: sessions handed
	// out warm, and sessions created cold because none was available.
	ClaimedTotal int `json:"claimedTotal,omitempty"`
	ColdTotal    int `json:"coldTotal,omitempty"`
	// LastError is the last machine-creation failure; ConfigError says
	// why the stored configuration no longer builds. Both empty when
	// healthy.
	LastError   string `json:"lastError,omitempty"`
	ConfigError string `json:"configError,omitempty"`
	// PausedUntil is until when the backend backs off after repeated
	// failures (RFC3339); empty when active.
	PausedUntil string `json:"pausedUntil,omitempty"`
	// Sessions is the inventory, oldest first. Filled by GetWarmPool
	// only; empty on list responses.
	Sessions []WarmPoolSession `json:"sessions,omitempty"`
}

// WarmPoolSession is one warm session in a pool's inventory.
type WarmPoolSession struct {
	SessionID string `json:"sessionId"`
	// State is "warming" or "ready".
	State     string `json:"state,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	// ReadyAt is when the session became ready; empty while warming.
	ReadyAt string `json:"readyAt,omitempty"`
}

// CreateWarmPoolRequest is the POST body for creating a warm pool. Apart
// from Name, OwnerType and DesiredCount, the fields are those of
// CreateSessionRequest that shape the VM, and the backend validates them
// the same way.
type CreateWarmPoolRequest struct {
	Name       string `json:"name"`
	TemplateID string `json:"templateId"`
	// OwnerType is "user" (the default for a personal token) or
	// "workspace" (shared; the only option for a Workspace API Token).
	// Sent only when set so the backend default applies otherwise.
	OwnerType string `json:"ownerType,omitempty"`
	// DesiredCount may be 0: an inert pool that still serves as a preset.
	DesiredCount            int                 `json:"desiredCount,omitempty"`
	SessionInputs           []SessionInputValue `json:"sessionInputs,omitempty"`
	MapSavedToSessionInputs bool                `json:"mapSavedToSessionInputs,omitempty"`
	EnabledFeatureFlagNames []string            `json:"enabledFeatureFlagNames,omitempty"`
	StackID                 string              `json:"stackId,omitempty"`
	MachineType             string              `json:"machineType,omitempty"`
	Cluster                 string              `json:"cluster,omitempty"`
	DeviceSpec              *DeviceSpec         `json:"deviceSpec,omitempty"`
	NoDevice                bool                `json:"noDevice,omitempty"`
}

// UpdateWarmPoolRequest is the PATCH body. Pointer scalars are sent only
// when non-nil (an empty string clears an override); the repeated and
// structured fields replace the server's list only when their UpdateXxx
// switch is true — UpdateTemplateRequest's convention. A configuration
// change invalidates the pool's inventory, which the backend rebuilds.
type UpdateWarmPoolRequest struct {
	Name         *string `json:"name,omitempty"`
	DesiredCount *int    `json:"desiredCount,omitempty"`

	SessionInputs                 []SessionInputValue `json:"sessionInputs,omitempty"`
	UpdateSessionInputs           bool                `json:"updateSessionInputs,omitempty"`
	EnabledFeatureFlagNames       []string            `json:"enabledFeatureFlagNames,omitempty"`
	UpdateEnabledFeatureFlagNames bool                `json:"updateEnabledFeatureFlagNames,omitempty"`

	StackID     *string `json:"stackId,omitempty"`
	MachineType *string `json:"machineType,omitempty"`
	Cluster     *string `json:"cluster,omitempty"`

	DeviceSpec       *DeviceSpec `json:"deviceSpec,omitempty"`
	UpdateDeviceSpec bool        `json:"updateDeviceSpec,omitempty"`
	NoDevice         *bool       `json:"noDevice,omitempty"`
}

type listWarmPoolsResp struct {
	WarmPools []WarmPool `json:"warmPools"`
}

type warmPoolResp struct {
	WarmPool WarmPool `json:"warmPool"`
}

// ListWarmPools returns the warm pools visible to the caller: the
// workspace's pools plus the caller's own user pools. templateID, when
// non-empty, narrows the list to pools of that template. all switches to
// the read-only every-pool-in-the-workspace view, which the backend gates
// on the workspace's billing-data permission.
// Endpoint: GET /v1/workspaces/{workspaceId}/warm-pools.
func (c *Client) ListWarmPools(ctx context.Context, workspaceID, templateID string, all bool) ([]WarmPool, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	p := wsPath(workspaceID, "/warm-pools")
	q := url.Values{}
	if templateID != "" {
		q.Set("templateId", templateID)
	}
	if all {
		q.Set("scope", "WARM_POOL_LIST_SCOPE_ALL")
	}
	if encoded := q.Encode(); encoded != "" {
		p += "?" + encoded
	}
	var resp listWarmPoolsResp
	if err := c.getJSON(ctx, p, &resp); err != nil {
		return nil, err
	}
	return resp.WarmPools, nil
}

// GetWarmPool returns a single warm pool with its live status, including
// the inventory (Status.Sessions).
// Endpoint: GET /v1/workspaces/{workspaceId}/warm-pools/{warmPoolId}.
func (c *Client) GetWarmPool(ctx context.Context, workspaceID, warmPoolID string) (WarmPool, error) {
	if workspaceID == "" {
		return WarmPool{}, fmt.Errorf("workspace ID is required")
	}
	if warmPoolID == "" {
		return WarmPool{}, fmt.Errorf("warm pool ID is required")
	}
	var resp warmPoolResp
	p := wsPath(workspaceID, "/warm-pools/"+url.PathEscape(warmPoolID))
	if err := c.getJSON(ctx, p, &resp); err != nil {
		return WarmPool{}, err
	}
	return resp.WarmPool, nil
}

// CreateWarmPool creates a warm pool in the workspace.
// Endpoint: POST /v1/workspaces/{workspaceId}/warm-pools.
func (c *Client) CreateWarmPool(ctx context.Context, workspaceID string, req CreateWarmPoolRequest) (WarmPool, error) {
	if workspaceID == "" {
		return WarmPool{}, fmt.Errorf("workspace ID is required")
	}
	var resp warmPoolResp
	if err := c.sendJSON(ctx, http.MethodPost, wsPath(workspaceID, "/warm-pools"), req, &resp); err != nil {
		return WarmPool{}, err
	}
	return resp.WarmPool, nil
}

// UpdateWarmPool patches a warm pool. See UpdateWarmPoolRequest for the
// UpdateXxx-switch semantics around the repeated fields.
// Endpoint: PATCH /v1/workspaces/{workspaceId}/warm-pools/{warmPoolId}.
func (c *Client) UpdateWarmPool(ctx context.Context, workspaceID, warmPoolID string, req UpdateWarmPoolRequest) (WarmPool, error) {
	if workspaceID == "" {
		return WarmPool{}, fmt.Errorf("workspace ID is required")
	}
	if warmPoolID == "" {
		return WarmPool{}, fmt.Errorf("warm pool ID is required")
	}
	var resp warmPoolResp
	p := wsPath(workspaceID, "/warm-pools/"+url.PathEscape(warmPoolID))
	if err := c.sendJSON(ctx, http.MethodPatch, p, req, &resp); err != nil {
		return WarmPool{}, err
	}
	return resp.WarmPool, nil
}

// DeleteWarmPool deletes a warm pool. Its warm sessions are terminated
// and removed by the backend; sessions already claimed are untouched.
// Endpoint: DELETE /v1/workspaces/{workspaceId}/warm-pools/{warmPoolId}.
func (c *Client) DeleteWarmPool(ctx context.Context, workspaceID, warmPoolID string) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace ID is required")
	}
	if warmPoolID == "" {
		return fmt.Errorf("warm pool ID is required")
	}
	return c.del(ctx, wsPath(workspaceID, "/warm-pools/"+url.PathEscape(warmPoolID)))
}
