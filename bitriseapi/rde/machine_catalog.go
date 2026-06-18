package rde

import (
	"context"
	"fmt"
)

// Image is a machine image available for templates/sessions.
type Image struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ClusterName string `json:"clusterName,omitempty"`
	// IsDefault is set by the backend on the deployment's default image.
	IsDefault bool `json:"isDefault,omitempty"`
}

// MachineType is a machine size available for templates/sessions.
type MachineType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ClusterName string `json:"clusterName,omitempty"`
	// IsDefault is set by the backend on the deployment's default machine type.
	IsDefault bool `json:"isDefault,omitempty"`
}

type listImagesResp struct {
	Images []Image `json:"images"`
}

type listMachineTypesResp struct {
	MachineTypes []MachineType `json:"machineTypes"`
}

// ListImages returns every machine image available to the workspace.
// Endpoint: GET /v1/workspaces/{workspaceId}/images.
func (c *Client) ListImages(ctx context.Context, workspaceID string) ([]Image, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	var resp listImagesResp
	if err := c.getJSON(ctx, wsPath(workspaceID, "/images"), &resp); err != nil {
		return nil, err
	}
	return resp.Images, nil
}

// ListMachineTypes returns every machine type available to the workspace.
// Endpoint: GET /v1/workspaces/{workspaceId}/machine-types.
func (c *Client) ListMachineTypes(ctx context.Context, workspaceID string) ([]MachineType, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	var resp listMachineTypesResp
	if err := c.getJSON(ctx, wsPath(workspaceID, "/machine-types"), &resp); err != nil {
		return nil, err
	}
	return resp.MachineTypes, nil
}
