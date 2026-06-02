package rde

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// SavedInput is a user-scoped credential/value reusable across sessions.
//
// Secret values (IsSecret=true) are only returned by the read endpoints
// (ListSavedInputs / GetSavedInput) when a request opts in with
// include_secrets=true (RDE-269). The CLI intentionally never sets that flag
// — it has no use for the cleartext (see the comment on ListSavedInputs) and
// the internal/rde mapper masks any secret value before the CLI hands saved
// inputs to renderers. So `Value` is empty for secret inputs on reads, and
// the mapper masks it again as defense-in-depth in case the backend default
// ever changes (create/update may still echo the just-submitted value back).
type SavedInput struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
	IsSecret  bool   `json:"isSecret,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// CreateSavedInputRequest is the POST body for creating a saved input.
type CreateSavedInputRequest struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsSecret bool   `json:"isSecret,omitempty"`
}

// UpdateSavedInputRequest is the PATCH body. Pointer fields preserve the
// "unset, leave alone" semantics — only fields with non-nil values are sent.
type UpdateSavedInputRequest struct {
	Value    *string `json:"value,omitempty"`
	IsSecret *bool   `json:"isSecret,omitempty"`
}

type listSavedInputsResp struct {
	SavedInputs []SavedInput `json:"savedInputs"`
}

type savedInputResp struct {
	SavedInput SavedInput `json:"savedInput"`
}

// Saved inputs are USER-scoped, not workspace-scoped — the path is
// /v1/saved-inputs (no /workspaces/{id} segment). Callers don't need a
// workspace ID for any of these.

// ListSavedInputs returns every saved input for the caller.
// Endpoint: GET /v1/saved-inputs.
//
// Deliberately does not pass include_secrets (RDE-269): its only consumer,
// the `saved-input list` table, prints "(hidden)" for secret inputs and only
// renders the value of non-secret ones, so the cleartext is unwanted (and
// would leak into --output json, shell history, and log files). Add the query
// param here (and thread it through internal/rde) only if a caller genuinely
// needs cleartext secrets, which would also mean revisiting the masking in
// savedInputFromAPI. Mirrors ListTemplates.
func (c *Client) ListSavedInputs(ctx context.Context) ([]SavedInput, error) {
	var resp listSavedInputsResp
	if err := c.getJSON(ctx, userPath("/saved-inputs"), &resp); err != nil {
		return nil, err
	}
	return resp.SavedInputs, nil
}

// GetSavedInput returns a saved input by ID.
// Endpoint: GET /v1/saved-inputs/{savedInputId}.
//
// Deliberately does not pass include_secrets (RDE-269): its only consumer,
// `saved-input view`, prints "(hidden)" for a secret input and only renders
// the value of a non-secret one, so the cleartext is unwanted. See the note
// on ListSavedInputs.
func (c *Client) GetSavedInput(ctx context.Context, id string) (SavedInput, error) {
	if id == "" {
		return SavedInput{}, fmt.Errorf("saved input ID is required")
	}
	var resp savedInputResp
	if err := c.getJSON(ctx, userPath("/saved-inputs/"+url.PathEscape(id)), &resp); err != nil {
		return SavedInput{}, err
	}
	return resp.SavedInput, nil
}

// CreateSavedInput creates a new saved input.
// Endpoint: POST /v1/saved-inputs.
func (c *Client) CreateSavedInput(ctx context.Context, req CreateSavedInputRequest) (SavedInput, error) {
	var resp savedInputResp
	if err := c.sendJSON(ctx, http.MethodPost, userPath("/saved-inputs"), req, &resp); err != nil {
		return SavedInput{}, err
	}
	return resp.SavedInput, nil
}

// UpdateSavedInput patches a saved input's value and/or secret flag.
// Endpoint: PATCH /v1/saved-inputs/{savedInputId}.
func (c *Client) UpdateSavedInput(ctx context.Context, id string, req UpdateSavedInputRequest) (SavedInput, error) {
	if id == "" {
		return SavedInput{}, fmt.Errorf("saved input ID is required")
	}
	var resp savedInputResp
	p := userPath("/saved-inputs/" + url.PathEscape(id))
	if err := c.sendJSON(ctx, http.MethodPatch, p, req, &resp); err != nil {
		return SavedInput{}, err
	}
	return resp.SavedInput, nil
}

// DeleteSavedInput removes a saved input.
// Endpoint: DELETE /v1/saved-inputs/{savedInputId}.
func (c *Client) DeleteSavedInput(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("saved input ID is required")
	}
	return c.del(ctx, userPath("/saved-inputs/"+url.PathEscape(id)))
}
