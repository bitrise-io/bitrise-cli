package rde

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Template is the wire-format template record. Phase 1 only reads templates
// (list, view); create/update/delete is deferred to Phase 3 along with the
// nested *Create variants.
type Template struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	StackID     string `json:"stackId,omitempty"`
	// Image is retained only as a read fallback for templates created before
	// stackId was populated.
	Image            string `json:"image,omitempty"`
	MachineType      string `json:"machineType,omitempty"`
	WorkingDirectory string `json:"workingDirectory,omitempty"`
	StartupScript    string `json:"startupScript,omitempty"`
	WarmupScript     string `json:"warmupScript,omitempty"`
	// LifecycleMode says how the template's scripts are stored and run:
	// "legacy" (StartupScript / WarmupScript, the platform's two boot
	// stages) or "hooks" (Hooks, run on the booted machine by the RDE
	// runner). On a "hooks" template StartupScript reads back as a
	// generated, read-only equivalent of the hooks.
	LifecycleMode     string             `json:"lifecycleMode,omitempty"`
	Hooks             *TemplateHooks     `json:"hooks,omitempty"`
	CreatedByEmail    string             `json:"createdByEmail,omitempty"`
	TemplateVariables []TemplateVariable `json:"templateVariables,omitempty"`
	SessionInputs     []SessionInputDef  `json:"sessionInputs,omitempty"`
	FeatureFlags      []FeatureFlag      `json:"featureFlags,omitempty"`
	WorkspaceLinks    []WorkspaceLink    `json:"workspaceLinks,omitempty"`
	// DeviceSpec is the virtual device (iOS simulator / Android emulator)
	// sessions created from this template boot by default; nil when the
	// template declares none.
	DeviceSpec *DeviceSpec `json:"deviceSpec,omitempty"`
	CreatedAt  string      `json:"createdAt,omitempty"`
	UpdatedAt  string      `json:"updatedAt,omitempty"`
}

// TemplateHooks are a template's lifecycle hook scripts (bash). On create
// runs once when the session is created (on a warm pool: when the pool warms
// the machine); On claim runs on every claim; On restore runs every time a
// stopped session is restored. nil means the template does not have the
// hook; an added hook runs even when its script is empty ("" finishes at
// once, exit 0).
type TemplateHooks struct {
	OnCreate  *string `json:"onCreate,omitempty"`
	OnClaim   *string `json:"onClaim,omitempty"`
	OnRestore *string `json:"onRestore,omitempty"`
}

// TemplateVariable is a baked-in template variable.
//
// Secret values (IsSecret=true) are only returned by the backend when a
// request opts in with include_secrets=true. The CLI intentionally
// never sets that flag on template reads — it has no use for the cleartext
// (see ListTemplates / GetTemplate) and the internal/rde mapper masks any
// secret value before the CLI hands templates to renderers. So `Value` is
// empty for secret variables, and the mapper masks it again as
// defense-in-depth in case the backend default ever changes.
type TemplateVariable struct {
	ID       string `json:"id,omitempty"`
	Key      string `json:"key"`
	Value    string `json:"value,omitempty"`
	IsSecret bool   `json:"isSecret,omitempty"`
	// ExposeAsEnvVar ("Also in terminal"): the value is also written to the
	// session's shell profile when its scope's step runs. Refused on a
	// protected variable.
	ExposeAsEnvVar bool `json:"exposeAsEnvVar,omitempty"`
	// Hook is the scope that receives the variable: "" or "shared" reaches
	// every hook; "on_create", "on_claim" or "on_restore" reach that hook
	// only (templates with lifecycle hooks). Keys are unique per scope; the
	// hook's own value wins over a shared value of the same key.
	Hook string `json:"hook,omitempty"`
	// Protected: write-only — the API never returns the value (reads give an
	// empty Value with IsSecret true, whatever include_secrets says); an
	// empty value on update keeps the stored one. Implies IsSecret.
	Protected bool `json:"protected,omitempty"`
}

// SessionInputDef defines an input the template prompts for at session
// creation time. (Distinct from SessionInputValue, which is the user's
// answer in a CreateSessionRequest.)
type SessionInputDef struct {
	ID             string `json:"id,omitempty"`
	Key            string `json:"key"`
	Description    string `json:"description,omitempty"`
	Required       bool   `json:"required,omitempty"`
	DefaultValue   string `json:"defaultValue,omitempty"`
	ExposeAsEnvVar bool   `json:"exposeAsEnvVar,omitempty"`
	// Hook is the scope that receives the input: "" or "shared" reaches
	// every hook and is supplied when the session or the pool is created;
	// "on_create" is supplied with the pool (or the direct create) and
	// reaches On create only; "on_claim" and "on_restore" are supplied by
	// whoever claims (or at a direct create) and reach that hook only.
	// Keys are unique per scope; the same key may exist on several scopes.
	Hook string `json:"hook,omitempty"`
}

// FeatureFlag is a toggleable feature on a template.
type FeatureFlag struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// WorkspaceLink is an IDE folder shortcut bundled with a template.
type WorkspaceLink struct {
	Label      string `json:"label,omitempty"`
	FolderPath string `json:"folderPath,omitempty"`
	SortOrder  int    `json:"sortOrder,omitempty"`
}

type listTemplatesResp struct {
	Templates []Template `json:"templates"`
}

type templateResp struct {
	Template Template `json:"template"`
}

// TemplateVariableCreate is the create-time shape of a template variable;
// distinct from TemplateVariable because the value is required at create
// (no ID is sent — the server assigns one).
type TemplateVariableCreate struct {
	Key            string `json:"key"`
	Value          string `json:"value,omitempty"`
	IsSecret       bool   `json:"isSecret,omitempty"`
	ExposeAsEnvVar bool   `json:"exposeAsEnvVar,omitempty"`
	// Hook: see TemplateVariable.Hook. Empty means shared; a per-hook scope
	// needs a template with lifecycle hooks.
	Hook string `json:"hook,omitempty"`
	// Protected: see TemplateVariable.Protected. An empty Value on update
	// keeps the stored one.
	Protected bool `json:"protected,omitempty"`
}

// SessionInputCreate is the create-time shape of a session input definition.
type SessionInputCreate struct {
	Key            string `json:"key"`
	Description    string `json:"description,omitempty"`
	Required       bool   `json:"required,omitempty"`
	DefaultValue   string `json:"defaultValue,omitempty"`
	ExposeAsEnvVar bool   `json:"exposeAsEnvVar,omitempty"`
	// Hook: see SessionInputDef.Hook. Empty means shared; a per-hook scope
	// needs a template with lifecycle hooks.
	Hook string `json:"hook,omitempty"`
}

// FeatureFlagCreate is the create-time shape of a feature flag.
type FeatureFlagCreate struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// WorkspaceLinkCreate is the create-time shape of a workspace link.
type WorkspaceLinkCreate struct {
	Label           string `json:"label,omitempty"`
	FolderPath      string `json:"folderPath,omitempty"`
	FeatureFlagName string `json:"featureFlagName,omitempty"`
}

// CreateTemplateRequest is the POST body for creating a template.
type CreateTemplateRequest struct {
	Name              string                   `json:"name"`
	Description       string                   `json:"description,omitempty"`
	StackID           string                   `json:"stackId"`
	MachineType       string                   `json:"machineType"`
	WorkingDirectory  string                   `json:"workingDirectory,omitempty"`
	StartupScript     string                   `json:"startupScript,omitempty"`
	WarmupScript      string                   `json:"warmupScript,omitempty"`
	TemplateVariables []TemplateVariableCreate `json:"templateVariables,omitempty"`
	SessionInputs     []SessionInputCreate     `json:"sessionInputs,omitempty"`
	FeatureFlags      []FeatureFlagCreate      `json:"featureFlags,omitempty"`
	WorkspaceLinks    []WorkspaceLinkCreate    `json:"workspaceLinks,omitempty"`
	// Hooks makes this a template with lifecycle hooks (cannot be combined
	// with StartupScript / WarmupScript; requires lifecycle hooks to be
	// available on the workspace).
	Hooks *TemplateHooks `json:"hooks,omitempty"`
	// DeviceSpec declares the device sessions created from the template boot
	// by default (optional).
	DeviceSpec *DeviceSpec `json:"deviceSpec,omitempty"`
}

// UpdateTemplateRequest is the PATCH body. The UpdateXxx booleans are
// required when an array field should replace the server's existing list,
// and UpdateDeviceSpec when the template's device should become DeviceSpec
// (nil DeviceSpec + UpdateDeviceSpec clears it).
type UpdateTemplateRequest struct {
	Name             *string `json:"name,omitempty"`
	Description      *string `json:"description,omitempty"`
	StackID          *string `json:"stackId,omitempty"`
	MachineType      *string `json:"machineType,omitempty"`
	WorkingDirectory *string `json:"workingDirectory,omitempty"`
	StartupScript    *string `json:"startupScript,omitempty"`
	WarmupScript     *string `json:"warmupScript,omitempty"`

	TemplateVariables       []TemplateVariableCreate `json:"templateVariables,omitempty"`
	UpdateTemplateVariables bool                     `json:"updateTemplateVariables,omitempty"`
	SessionInputs           []SessionInputCreate     `json:"sessionInputs,omitempty"`
	UpdateSessionInputs     bool                     `json:"updateSessionInputs,omitempty"`
	FeatureFlags            []FeatureFlagCreate      `json:"featureFlags,omitempty"`
	UpdateFeatureFlags      bool                     `json:"updateFeatureFlags,omitempty"`
	WorkspaceLinks          []WorkspaceLinkCreate    `json:"workspaceLinks,omitempty"`
	UpdateWorkspaceLinks    bool                     `json:"updateWorkspaceLinks,omitempty"`
	DeviceSpec              *DeviceSpec              `json:"deviceSpec,omitempty"`
	UpdateDeviceSpec        bool                     `json:"updateDeviceSpec,omitempty"`
	// Hooks replaces the template's lifecycle hooks as a whole when
	// UpdateHooks is set (and turns a legacy template into a hooks template).
	Hooks       *TemplateHooks `json:"hooks,omitempty"`
	UpdateHooks bool           `json:"updateHooks,omitempty"`
}

// ListTemplates returns every template visible in the workspace.
// Endpoint: GET /v1/workspaces/{workspaceId}/templates.
//
// Deliberately does not pass include_secrets: the only consumers —
// the `template list` table and ResolveTemplateID's name→ID lookup — read
// metadata only, never secret variable values. Add the query param here (and
// thread it through internal/rde) only if a caller genuinely needs cleartext
// secrets, which would also mean revisiting the masking in templateFromAPI.
func (c *Client) ListTemplates(ctx context.Context, workspaceID string) ([]Template, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	var resp listTemplatesResp
	if err := c.getJSON(ctx, wsPath(workspaceID, "/templates"), &resp); err != nil {
		return nil, err
	}
	return resp.Templates, nil
}

// GetTemplate returns a single template by ID.
// Endpoint: GET /v1/workspaces/{workspaceId}/templates/{templateId}.
//
// Deliberately does not pass include_secrets: its only consumer,
// `template view`, prints each secret variable's key with a "(secret)" label
// but never its value, so the cleartext is unwanted (and would leak into
// stdout / JSON output / shell history). See the note on ListTemplates.
func (c *Client) GetTemplate(ctx context.Context, workspaceID, templateID string) (Template, error) {
	if workspaceID == "" {
		return Template{}, fmt.Errorf("workspace ID is required")
	}
	if templateID == "" {
		return Template{}, fmt.Errorf("template ID is required")
	}
	var resp templateResp
	p := wsPath(workspaceID, "/templates/"+url.PathEscape(templateID))
	if err := c.getJSON(ctx, p, &resp); err != nil {
		return Template{}, err
	}
	return resp.Template, nil
}

// CreateTemplate creates a new template in the workspace.
// Endpoint: POST /v1/workspaces/{workspaceId}/templates.
func (c *Client) CreateTemplate(ctx context.Context, workspaceID string, req CreateTemplateRequest) (Template, error) {
	if workspaceID == "" {
		return Template{}, fmt.Errorf("workspace ID is required")
	}
	var resp templateResp
	if err := c.sendJSON(ctx, http.MethodPost, wsPath(workspaceID, "/templates"), req, &resp); err != nil {
		return Template{}, err
	}
	return resp.Template, nil
}

// UpdateTemplate patches an existing template. See UpdateTemplateRequest
// for the UpdateXxx-boolean semantics around the array fields.
// Endpoint: PATCH /v1/workspaces/{workspaceId}/templates/{templateId}.
func (c *Client) UpdateTemplate(ctx context.Context, workspaceID, templateID string, req UpdateTemplateRequest) (Template, error) {
	if workspaceID == "" {
		return Template{}, fmt.Errorf("workspace ID is required")
	}
	if templateID == "" {
		return Template{}, fmt.Errorf("template ID is required")
	}
	var resp templateResp
	p := wsPath(workspaceID, "/templates/"+url.PathEscape(templateID))
	if err := c.sendJSON(ctx, http.MethodPatch, p, req, &resp); err != nil {
		return Template{}, err
	}
	return resp.Template, nil
}

// DeleteTemplate removes a template (soft-delete server-side).
// Endpoint: DELETE /v1/workspaces/{workspaceId}/templates/{templateId}.
func (c *Client) DeleteTemplate(ctx context.Context, workspaceID, templateID string) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace ID is required")
	}
	if templateID == "" {
		return fmt.Errorf("template ID is required")
	}
	return c.del(ctx, wsPath(workspaceID, "/templates/"+url.PathEscape(templateID)))
}
