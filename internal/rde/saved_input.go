package rde

import (
	"context"
	"time"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

// SavedInput is the CLI-facing saved-input record. `Value` is masked
// (server-side) when IsSecret=true.
type SavedInput struct {
	ID        string     `json:"id"`
	Key       string     `json:"key"`
	Value     string     `json:"value,omitempty"`
	IsSecret  bool       `json:"is_secret,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// CreateSavedInputRequest is the CLI-side create payload.
type CreateSavedInputRequest struct {
	Key      string
	Value    string
	IsSecret bool
}

// UpdateSavedInputRequest is the CLI-side patch payload. Pointer fields
// preserve "unset, leave alone" semantics.
type UpdateSavedInputRequest struct {
	Value    *string
	IsSecret *bool
}

// ListSavedInputs returns every saved input for the caller. Saved inputs
// are user-scoped, not workspace-scoped — no workspace ID needed.
func (s *Service) ListSavedInputs(ctx context.Context) ([]SavedInput, error) {
	if s.client == nil {
		return nil, errClient()
	}
	wire, err := s.client.ListSavedInputs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SavedInput, 0, len(wire))
	for _, w := range wire {
		out = append(out, savedInputFromAPI(w))
	}
	return out, nil
}

// GetSavedInput returns a saved input by ID.
func (s *Service) GetSavedInput(ctx context.Context, id string) (SavedInput, error) {
	if s.client == nil {
		return SavedInput{}, errClient()
	}
	w, err := s.client.GetSavedInput(ctx, id)
	if err != nil {
		return SavedInput{}, err
	}
	return savedInputFromAPI(w), nil
}

// CreateSavedInput creates a saved input.
func (s *Service) CreateSavedInput(ctx context.Context, req CreateSavedInputRequest) (SavedInput, error) {
	if s.client == nil {
		return SavedInput{}, errClient()
	}
	w, err := s.client.CreateSavedInput(ctx, rdeapi.CreateSavedInputRequest{
		Key:      req.Key,
		Value:    req.Value,
		IsSecret: req.IsSecret,
	})
	if err != nil {
		return SavedInput{}, err
	}
	return savedInputFromAPI(w), nil
}

// UpdateSavedInput patches a saved input.
func (s *Service) UpdateSavedInput(ctx context.Context, id string, req UpdateSavedInputRequest) (SavedInput, error) {
	if s.client == nil {
		return SavedInput{}, errClient()
	}
	w, err := s.client.UpdateSavedInput(ctx, id, rdeapi.UpdateSavedInputRequest{
		Value:    req.Value,
		IsSecret: req.IsSecret,
	})
	if err != nil {
		return SavedInput{}, err
	}
	return savedInputFromAPI(w), nil
}

// DeleteSavedInput removes a saved input.
func (s *Service) DeleteSavedInput(ctx context.Context, id string) error {
	if s.client == nil {
		return errClient()
	}
	return s.client.DeleteSavedInput(ctx, id)
}

func savedInputFromAPI(w rdeapi.SavedInput) SavedInput {
	return SavedInput{
		ID:        w.ID,
		Key:       w.Key,
		Value:     w.Value,
		IsSecret:  w.IsSecret,
		CreatedAt: parseTime(w.CreatedAt),
		UpdatedAt: parseTime(w.UpdatedAt),
	}
}
