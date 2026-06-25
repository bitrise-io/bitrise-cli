package rde

import (
	"context"
	"fmt"
)

// Stack is a machine stack available for templates/sessions. The id is the
// stable contract stored on a template/session; the remaining fields are
// human-friendly catalog metadata.
type Stack struct {
	ID           string `json:"id"`
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
	OS           string `json:"os,omitempty"`
	OSVersion    int32  `json:"osVersion,omitempty"`
	Status       string `json:"status,omitempty"`
	XcodeVersion string `json:"xcodeVersion,omitempty"`
	// IsDefault is set by the backend on the deployment's default stack.
	IsDefault bool `json:"isDefault,omitempty"`
	// ClusterNames are the clusters where this stack can be provisioned.
	ClusterNames []string `json:"clusterNames,omitempty"`
	// DescriptionLink points at the stack's pre-installed tools / system report.
	DescriptionLink string `json:"descriptionLink,omitempty"`
}

// MachineType is a machine size available for templates/sessions.
type MachineType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ClusterName string `json:"clusterName,omitempty"`
	// IsDefault is set by the backend on the deployment's default machine type.
	IsDefault bool `json:"isDefault,omitempty"`
}

type listStacksResp struct {
	Stacks []Stack `json:"stacks"`
}

type listMachineTypesResp struct {
	MachineTypes []MachineType `json:"machineTypes"`
}

// ListStacks returns every machine stack available to the workspace.
// Endpoint: GET /v1/workspaces/{workspaceId}/stacks.
func (c *Client) ListStacks(ctx context.Context, workspaceID string) ([]Stack, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	var resp listStacksResp
	if err := c.getJSON(ctx, wsPath(workspaceID, "/stacks"), &resp); err != nil {
		return nil, err
	}
	return resp.Stacks, nil
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
