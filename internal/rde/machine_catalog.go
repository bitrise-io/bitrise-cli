package rde

import (
	"context"
)

// Image is a machine image available in the workspace.
type Image struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ClusterName string `json:"cluster_name,omitempty"`
}

// MachineType is a machine size available in the workspace.
type MachineType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ClusterName string `json:"cluster_name,omitempty"`
}

// ListImages returns every machine image available in the workspace.
func (s *Service) ListImages(ctx context.Context, workspaceID string) ([]Image, error) {
	if s.client == nil {
		return nil, errClient()
	}
	wire, err := s.client.ListImages(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]Image, 0, len(wire))
	for _, w := range wire {
		out = append(out, Image{ID: w.ID, Name: w.Name, ClusterName: w.ClusterName})
	}
	return out, nil
}

// ListMachineTypes returns every machine type available in the workspace.
func (s *Service) ListMachineTypes(ctx context.Context, workspaceID string) ([]MachineType, error) {
	if s.client == nil {
		return nil, errClient()
	}
	wire, err := s.client.ListMachineTypes(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]MachineType, 0, len(wire))
	for _, w := range wire {
		out = append(out, MachineType{ID: w.ID, Name: w.Name, ClusterName: w.ClusterName})
	}
	return out, nil
}
