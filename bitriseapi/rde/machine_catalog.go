package rde

import (
	"context"
	"fmt"
	"net/http"
)

// Image is a machine image available for templates/sessions.
type Image struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ClusterName string `json:"clusterName,omitempty"`
}

// MachineType is a machine size available for templates/sessions.
type MachineType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ClusterName string `json:"clusterName,omitempty"`
}

// ClusterOption is one cluster that offers a given image + machine type.
// `ImageID` and `MachineTypeID` are cluster-specific, useful for the
// backend's downstream provisioning calls but not surfaced to CLI users.
type ClusterOption struct {
	ClusterName   string `json:"clusterName"`
	ImageID       string `json:"imageId,omitempty"`
	MachineTypeID string `json:"machineTypeId,omitempty"`
}

// ResolveClustersRequest asks which clusters offer both Image and MachineType.
type ResolveClustersRequest struct {
	Image       string `json:"image"`
	MachineType string `json:"machineType"`
}

type listImagesResp struct {
	Images []Image `json:"images"`
}

type listMachineTypesResp struct {
	MachineTypes []MachineType `json:"machineTypes"`
}

type resolveClustersResp struct {
	Clusters []ClusterOption `json:"clusters"`
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

// ResolveClusters finds clusters that serve both image and machineType.
// Returns an empty slice when no cluster matches.
// Endpoint: POST /v1/workspaces/{workspaceId}/resolve-clusters.
func (c *Client) ResolveClusters(ctx context.Context, workspaceID string, req ResolveClustersRequest) ([]ClusterOption, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if req.Image == "" {
		return nil, fmt.Errorf("image is required")
	}
	if req.MachineType == "" {
		return nil, fmt.Errorf("machine type is required")
	}
	var resp resolveClustersResp
	if err := c.sendJSON(ctx, http.MethodPost, wsPath(workspaceID, "/resolve-clusters"), req, &resp); err != nil {
		return nil, err
	}
	return resp.Clusters, nil
}
