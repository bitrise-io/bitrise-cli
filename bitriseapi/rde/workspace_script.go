package rde

import (
	"context"
	"fmt"
	"net/http"
)

// WorkspaceScript is the wire-format workspace script: one bash script per
// workspace that every session boot runs first, with its variables. The
// backend omits false booleans, empty strings and empty lists, so absent
// means the zero value.
type WorkspaceScript struct {
	// Configured is false until the first save: nothing is stored yet.
	Configured bool `json:"configured,omitempty"`
	// Enabled: sessions run the script. Disabled is draft mode — only
	// sessions created with CreateSessionRequest.WorkspaceScriptTest run it.
	Enabled   bool                      `json:"enabled,omitempty"`
	Script    string                    `json:"script,omitempty"`
	Variables []WorkspaceScriptVariable `json:"variables,omitempty"`
	// UpdatedBy is the email of the last editor; UpdatedAt (RFC3339) when
	// it was last saved. Both empty until configured.
	UpdatedBy string `json:"updatedBy,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// WorkspaceScriptVariable is one variable as the API returns it. Value is
// empty for a secret unless the request asked for secrets, and always empty
// for a protected (write-only) variable; ValueStored says whether a
// protected variable has a stored value.
type WorkspaceScriptVariable struct {
	Key         string `json:"key"`
	Value       string `json:"value,omitempty"`
	IsSecret    bool   `json:"isSecret,omitempty"`
	Protected   bool   `json:"protected,omitempty"`
	ValueStored bool   `json:"valueStored,omitempty"`
}

// WorkspaceScriptVariableInput is one variable as the PUT body sends it.
// Protected implies IsSecret. A protected variable sent with an empty Value
// keeps its stored value (same key, already protected).
type WorkspaceScriptVariableInput struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	IsSecret  bool   `json:"isSecret"`
	Protected bool   `json:"protected"`
}

// UpdateWorkspaceScriptRequest is the PUT body. It is a full replacement:
// a variable left out of Variables is deleted, so every field is always
// sent.
type UpdateWorkspaceScriptRequest struct {
	Enabled   bool                           `json:"enabled"`
	Script    string                         `json:"script"`
	Variables []WorkspaceScriptVariableInput `json:"variables"`
}

type workspaceScriptResp struct {
	WorkspaceScript *WorkspaceScript `json:"workspaceScript"`
}

func (r workspaceScriptResp) value() WorkspaceScript {
	if r.WorkspaceScript == nil {
		return WorkspaceScript{}
	}
	return *r.WorkspaceScript
}

// GetWorkspaceScript returns the workspace script. includeSecrets returns
// secret values in clear; protected values are never returned. Gated on
// the workspace's Usage permission and on the feature being available for
// the workspace (403 otherwise); Workspace API Tokens are refused.
// Endpoint: GET /v1/workspaces/{workspaceId}/workspace-script.
func (c *Client) GetWorkspaceScript(ctx context.Context, workspaceID string, includeSecrets bool) (WorkspaceScript, error) {
	if workspaceID == "" {
		return WorkspaceScript{}, fmt.Errorf("workspace ID is required")
	}
	p := wsPath(workspaceID, "/workspace-script")
	if includeSecrets {
		p += "?include_secrets=true"
	}
	var resp workspaceScriptResp
	if err := c.getJSON(ctx, p, &resp); err != nil {
		return WorkspaceScript{}, err
	}
	return resp.value(), nil
}

// UpdateWorkspaceScript saves the workspace script as a whole — script,
// enabled state and the complete set of variables — and returns it as
// saved (secret values masked). Same permission as GetWorkspaceScript.
// Endpoint: PUT /v1/workspaces/{workspaceId}/workspace-script.
func (c *Client) UpdateWorkspaceScript(ctx context.Context, workspaceID string, req UpdateWorkspaceScriptRequest) (WorkspaceScript, error) {
	if workspaceID == "" {
		return WorkspaceScript{}, fmt.Errorf("workspace ID is required")
	}
	if req.Variables == nil {
		req.Variables = []WorkspaceScriptVariableInput{}
	}
	var resp workspaceScriptResp
	if err := c.sendJSON(ctx, http.MethodPut, wsPath(workspaceID, "/workspace-script"), req, &resp); err != nil {
		return WorkspaceScript{}, err
	}
	return resp.value(), nil
}
