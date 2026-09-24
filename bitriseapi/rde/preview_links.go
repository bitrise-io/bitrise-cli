package rde

import (
	"context"
	"fmt"
)

// CreatePreviewLinkRequest mints a device preview link. DeviceSpec and
// Artifact are both required by the backend: a link always names the device
// to boot and the app to install on it.
//
// TTLSeconds is an int64 on the wire; protojson accepts it as a JSON number
// or a string, so the plain number this encodes to is fine.
type CreatePreviewLinkRequest struct {
	DeviceSpec *DeviceSpec     `json:"deviceSpec,omitempty"`
	Artifact   *DeviceArtifact `json:"artifact,omitempty"`
	// TTLSeconds is the link's lifetime. 0 uses the default of 24 hours.
	TTLSeconds int64 `json:"ttlSeconds,omitempty"`
	// StackID and MachineType override the platform defaults the link's
	// sessions would otherwise run on. Empty = the default.
	StackID     string `json:"stackId,omitempty"`
	MachineType string `json:"machineType,omitempty"`
	// SessionAutoTerminateMinutes is the idle window of the sessions the link
	// spawns. 0 uses the default of 60 minutes.
	SessionAutoTerminateMinutes int `json:"sessionAutoTerminateMinutes,omitempty"`
	// WarmPoolID serves the link's opens from a warm pool (see WarmPool):
	// each open claims a warm session when one is available and creates
	// one from the pool's configuration otherwise. The pool must be
	// workspace-owned and boot a device; it fixes the machine and the
	// device, so StackID and MachineType must be empty and DeviceSpec
	// empty or equal to the pool's.
	WarmPoolID string `json:"warmPoolId,omitempty"`
}

// PreviewLink is a minted device preview link. Nothing is stored server-side
// — this response is the only artifact, and the link cannot be read back or
// revoked, only left to expire.
type PreviewLink struct {
	// Token is the signed link, and on its own a bearer credential.
	Token string `json:"token"`
	// URL is the shareable viewer link.
	URL string `json:"url"`
	// JTI is the link's id, recorded on every session it spawns.
	JTI string `json:"jti"`
	// ExpiresAt is RFC3339.
	ExpiresAt string `json:"expiresAt"`
}

// CreatePreviewLink mints a device preview link for an app build.
// Endpoint: POST /v1/workspaces/{workspaceId}/preview-links.
func (c *Client) CreatePreviewLink(ctx context.Context, workspaceID string, req CreatePreviewLinkRequest) (PreviewLink, error) {
	if workspaceID == "" {
		return PreviewLink{}, fmt.Errorf("workspace ID is required")
	}
	var out PreviewLink
	if err := c.sendJSON(ctx, "POST", wsPath(workspaceID, "/preview-links"), req, &out); err != nil {
		return PreviewLink{}, err
	}
	return out, nil
}
