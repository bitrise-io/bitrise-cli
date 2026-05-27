package rde

import (
	"context"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
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

// ClusterOption is one cluster offering a given image + machine type.
type ClusterOption struct {
	ClusterName   string `json:"cluster_name"`
	ImageID       string `json:"image_id,omitempty"`
	MachineTypeID string `json:"machine_type_id,omitempty"`
}

// ResolveClustersRequest carries the inputs to ResolveClusters.
type ResolveClustersRequest struct {
	Image       string
	MachineType string
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

// ResolveClusters finds clusters that serve both image and machineType.
func (s *Service) ResolveClusters(ctx context.Context, workspaceID string, req ResolveClustersRequest) ([]ClusterOption, error) {
	if s.client == nil {
		return nil, errClient()
	}
	wire, err := s.client.ResolveClusters(ctx, workspaceID, rdeapi.ResolveClustersRequest{
		Image:       req.Image,
		MachineType: req.MachineType,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ClusterOption, 0, len(wire))
	for _, w := range wire {
		out = append(out, ClusterOption{
			ClusterName:   w.ClusterName,
			ImageID:       w.ImageID,
			MachineTypeID: w.MachineTypeID,
		})
	}
	return out, nil
}
