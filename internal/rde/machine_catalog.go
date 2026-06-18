package rde

import (
	"context"
	"fmt"
)

// Image is a machine image available in the workspace.
type Image struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ClusterName string `json:"cluster_name,omitempty"`
	// IsDefault is set by the backend on the deployment's default image.
	IsDefault bool `json:"is_default,omitempty"`
}

// MachineType is a machine size available in the workspace.
type MachineType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ClusterName string `json:"cluster_name,omitempty"`
	// IsDefault is set by the backend on the deployment's default machine type.
	IsDefault bool `json:"is_default,omitempty"`
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
		out = append(out, Image{ID: w.ID, Name: w.Name, ClusterName: w.ClusterName, IsDefault: w.IsDefault})
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
		out = append(out, MachineType{ID: w.ID, Name: w.Name, ClusterName: w.ClusterName, IsDefault: w.IsDefault})
	}
	return out, nil
}

// MachineTypesForImage returns the machine types whose cluster overlaps with
// the clusters offering the image named imageName. An image is offered by one
// or more clusters; a machine type is compatible when it's offered by at least
// one of those same clusters. Mirrors the FE's client-side join (see
// frontend/src/hooks/useClusterFiltering.ts).
//
// It errors if imageName isn't available in the workspace.
func (s *Service) MachineTypesForImage(ctx context.Context, workspaceID, imageName string) ([]MachineType, error) {
	images, err := s.ListImages(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	imageClusters := make(map[string]struct{})
	for _, im := range images {
		if im.Name == imageName {
			imageClusters[im.ClusterName] = struct{}{}
		}
	}
	if len(imageClusters) == 0 {
		return nil, fmt.Errorf("image %q not found in this workspace", imageName)
	}
	machineTypes, err := s.ListMachineTypes(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]MachineType, 0, len(machineTypes))
	for _, mt := range machineTypes {
		if _, ok := imageClusters[mt.ClusterName]; ok {
			out = append(out, mt)
		}
	}
	return out, nil
}
