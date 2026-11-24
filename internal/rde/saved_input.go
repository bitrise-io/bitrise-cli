package rde

import (
	"context"
	"regexp"
	"time"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

// SavedInput is the CLI-facing saved-input record. `Value` is masked by
// savedInputFromAPI when IsSecret=true — secrets are write-only, the backend
// never returns them, and the CLI blanks any value it does return before any
// renderer sees it. MaskedValue is what a secret displays as.
type SavedInput struct {
	ID       string `json:"id"`
	Key      string `json:"key"`
	Value    string `json:"value,omitempty"`
	IsSecret bool   `json:"is_secret,omitempty"`
	// MaskedValue is the masked view of a secret value, for display: "****"
	// followed by its last 4 characters when it is longer than 16 characters
	// (e.g. "****5678"); never the length. Empty for a plain value.
	MaskedValue string     `json:"masked_value,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// CreateSavedInputRequest is the CLI-side create payload.
type CreateSavedInputRequest struct {
	Key      string
	Value    string
	IsSecret bool
}

// UpdateSavedInputRequest is the CLI-side patch payload. Pointer fields
// preserve "unset, leave alone" semantics. Without a value the stored one is
// kept, except that turning a secret plain needs a new value.
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
	// Mask secret values at the CLI boundary: passing them through would
	// leak the value into --output json, shell history, and log files. Keep
	// the key + is_secret marker so callers can still see what was set.
	// Secrets are write-only, so the backend never returns these values —
	// this masking is the belt-and-suspenders second line of defense.
	// Mirrors templateFromAPI / snapshotFromAPI.
	val := w.Value
	if w.IsSecret {
		val = ""
	}
	return SavedInput{
		ID:          w.ID,
		Key:         w.Key,
		Value:       val,
		IsSecret:    w.IsSecret,
		MaskedValue: maskedValue(w.IsSecret, w.MaskedValue),
		CreatedAt:   parseTime(w.CreatedAt),
		UpdatedAt:   parseTime(w.UpdatedAt),
	}
}

// secretMask is what a secret displays as when the backend sent no masked
// value (an older backend, or nothing stored): the bare start of the
// backend's own mask.
const secretMask = "****"

// backendMask matches the backend's masked_value: "****" followed by the
// last 4 characters of a value longer than 16, e.g. "****5678", or just
// "****".
var backendMask = regexp.MustCompile(`^\*\*\*\*(.{4})?$`)

// maskedValue is the masked view the CLI keeps for a value: the backend's
// masked_value for a secret (never anything that is not a mask), nothing
// for a plain value.
func maskedValue(isSecret bool, wire string) string {
	if !isSecret {
		return ""
	}
	if !backendMask.MatchString(wire) {
		return secretMask
	}
	return wire
}
