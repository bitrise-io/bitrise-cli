package rde

import (
	"context"
	"fmt"
	"strings"
	"time"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

// WorkspaceScript is the CLI-facing workspace script. JSON tags define the
// stable `--output json` shape.
//
// The workspace script is one bash script per workspace that every session
// boot runs first — before the template's lifecycle hooks, on every
// session and every boot. Disabled is draft mode, not
// off: the script stays saved and runs only on test sessions
// (`session create --workspace-script-test`).
type WorkspaceScript struct {
	// Configured is false until the first save: nothing is stored yet.
	Configured bool                      `json:"configured"`
	Enabled    bool                      `json:"enabled"`
	Script     string                    `json:"script"`
	Variables  []WorkspaceScriptVariable `json:"variables"`
	// UpdatedBy is the email of the last editor; UpdatedAt when it was last
	// saved. Both omitted until configured.
	UpdatedBy string     `json:"updated_by,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// Workspace script variable visibilities, as WorkspaceScriptVariable.Visibility
// reports them.
const (
	// VariableVisibilityPlain values are stored and returned as is.
	VariableVisibilityPlain = "plain"
	// VariableVisibilitySecret values are encrypted at rest and returned
	// only when secrets are asked for.
	VariableVisibilitySecret = "secret"
	// VariableVisibilityProtected values are encrypted and write-only:
	// never returned.
	VariableVisibilityProtected = "protected"
)

// WorkspaceScriptVariable is one variable of the workspace script.
// Visibility is plain, secret or protected. Value is empty for a secret
// unless secrets were asked for, and always empty for a protected variable;
// ValueStored says whether a protected variable has a stored value.
type WorkspaceScriptVariable struct {
	Key         string `json:"key"`
	Value       string `json:"value,omitempty"`
	Visibility  string `json:"visibility"`
	ValueStored bool   `json:"value_stored,omitempty"`
}

// WorkspaceScriptVariableChange sets one variable: a new one is added, an
// existing one gets this value and visibility.
type WorkspaceScriptVariableChange struct {
	Key        string
	Value      string
	Visibility string
}

// WorkspaceScriptChanges is a partial update of the workspace script. Nil
// pointers and empty lists leave that part as it is.
type WorkspaceScriptChanges struct {
	Script          *string
	Enabled         *bool
	SetVariables    []WorkspaceScriptVariableChange
	RemoveVariables []string
}

// IsEmpty reports whether the changes would change nothing.
func (c WorkspaceScriptChanges) IsEmpty() bool {
	return c.Script == nil && c.Enabled == nil && len(c.SetVariables) == 0 && len(c.RemoveVariables) == 0
}

// Validate checks the changes on their own, without the stored script: it
// rejects an empty change set, empty and reserved keys, a variable set twice
// or both set and removed, an empty protected value and an unknown
// visibility. Callers run it before any request, so a flag mistake never
// waits on the network. Errors name keys, never values. Whether a removed
// variable exists can only be checked against the stored script, by
// UpdateWorkspaceScript.
func (c WorkspaceScriptChanges) Validate() error {
	if c.IsEmpty() {
		return fmt.Errorf("nothing to update")
	}
	set := make(map[string]bool, len(c.SetVariables))
	for _, ch := range c.SetVariables {
		if err := validateVariableKey(ch.Key); err != nil {
			return err
		}
		if set[ch.Key] {
			return fmt.Errorf("variable %s is set more than once", ch.Key)
		}
		switch ch.Visibility {
		case VariableVisibilityPlain, VariableVisibilitySecret:
		case VariableVisibilityProtected:
			if ch.Value == "" {
				return fmt.Errorf("protected variable %s needs a value", ch.Key)
			}
		default:
			return fmt.Errorf("variable %s: unknown visibility %q", ch.Key, ch.Visibility)
		}
		set[ch.Key] = true
	}
	for _, key := range c.RemoveVariables {
		if key == "" {
			return fmt.Errorf("variable key to remove must not be empty")
		}
		if set[key] {
			return fmt.Errorf("variable %s is both set and removed", key)
		}
	}
	return nil
}

// reservedVariablePrefix starts the names the platform sets itself.
const reservedVariablePrefix = "RDE_"

// GetWorkspaceScript returns the workspace script. includeSecrets returns
// secret values in clear; protected values are never returned.
func (s *Service) GetWorkspaceScript(ctx context.Context, workspaceID string, includeSecrets bool) (WorkspaceScript, error) {
	if s.client == nil {
		return WorkspaceScript{}, errClient()
	}
	wire, err := s.client.GetWorkspaceScript(ctx, workspaceID, includeSecrets)
	if err != nil {
		return WorkspaceScript{}, err
	}
	return workspaceScriptFromAPI(wire), nil
}

// UpdateWorkspaceScript applies changes to the workspace script. The API
// replaces the script as a whole, so this reads the current one (secret
// values included), applies only the given changes and saves the full set:
// untouched variables are sent back as they are, untouched protected ones
// with an empty value so they keep their stored value. Returns the script
// as saved (secret values masked).
func (s *Service) UpdateWorkspaceScript(ctx context.Context, workspaceID string, changes WorkspaceScriptChanges) (WorkspaceScript, error) {
	if s.client == nil {
		return WorkspaceScript{}, errClient()
	}
	if err := changes.Validate(); err != nil {
		return WorkspaceScript{}, err
	}
	current, err := s.client.GetWorkspaceScript(ctx, workspaceID, true)
	if err != nil {
		return WorkspaceScript{}, err
	}
	req, err := applyWorkspaceScriptChanges(current, changes)
	if err != nil {
		return WorkspaceScript{}, err
	}
	saved, err := s.client.UpdateWorkspaceScript(ctx, workspaceID, req)
	if err != nil {
		return WorkspaceScript{}, err
	}
	return workspaceScriptFromAPI(saved), nil
}

// applyWorkspaceScriptChanges builds the full-replacement PUT body from the
// current script (read with secrets) and changes that passed Validate. The
// one check left for here is that a removed variable exists. Variables keep
// their order; new ones are appended in the order given.
func applyWorkspaceScriptChanges(current rdeapi.WorkspaceScript, changes WorkspaceScriptChanges) (rdeapi.UpdateWorkspaceScriptRequest, error) {
	req := rdeapi.UpdateWorkspaceScriptRequest{
		Enabled: current.Enabled,
		Script:  current.Script,
	}
	if changes.Enabled != nil {
		req.Enabled = *changes.Enabled
	}
	if changes.Script != nil {
		req.Script = *changes.Script
	}

	set := make(map[string]WorkspaceScriptVariableChange, len(changes.SetVariables))
	for _, ch := range changes.SetVariables {
		set[ch.Key] = ch
	}
	existing := make(map[string]bool, len(current.Variables))
	for _, v := range current.Variables {
		existing[v.Key] = true
	}
	removed := map[string]bool{}
	for _, key := range changes.RemoveVariables {
		if !existing[key] {
			return rdeapi.UpdateWorkspaceScriptRequest{}, fmt.Errorf("the workspace script has no variable %s to remove", key)
		}
		removed[key] = true
	}

	req.Variables = make([]rdeapi.WorkspaceScriptVariableInput, 0, len(current.Variables)+len(changes.SetVariables))
	for _, v := range current.Variables {
		if removed[v.Key] {
			continue
		}
		if ch, ok := set[v.Key]; ok {
			req.Variables = append(req.Variables, variableInput(ch))
			delete(set, v.Key)
			continue
		}
		// Unchanged: a protected value is never returned, so an empty value
		// tells the backend to keep the stored one; a secret was read in
		// clear and goes back as it is.
		in := rdeapi.WorkspaceScriptVariableInput{Key: v.Key, Value: v.Value, IsSecret: v.IsSecret || v.Protected, Protected: v.Protected}
		if v.Protected {
			in.Value = ""
		}
		req.Variables = append(req.Variables, in)
	}
	for _, ch := range changes.SetVariables {
		if _, isNew := set[ch.Key]; isNew {
			req.Variables = append(req.Variables, variableInput(ch))
		}
	}
	return req, nil
}

func variableInput(ch WorkspaceScriptVariableChange) rdeapi.WorkspaceScriptVariableInput {
	return rdeapi.WorkspaceScriptVariableInput{
		Key:       ch.Key,
		Value:     ch.Value,
		IsSecret:  ch.Visibility != VariableVisibilityPlain,
		Protected: ch.Visibility == VariableVisibilityProtected,
	}
}

// validateVariableKey fails fast on what the backend is certain to refuse.
func validateVariableKey(key string) error {
	if key == "" {
		return fmt.Errorf("variable key must not be empty")
	}
	if strings.HasPrefix(key, reservedVariablePrefix) {
		return fmt.Errorf("variable %s: keys starting with %s are reserved", key, reservedVariablePrefix)
	}
	return nil
}

func workspaceScriptFromAPI(w rdeapi.WorkspaceScript) WorkspaceScript {
	out := WorkspaceScript{
		Configured: w.Configured,
		Enabled:    w.Enabled,
		Script:     w.Script,
		Variables:  make([]WorkspaceScriptVariable, 0, len(w.Variables)),
		UpdatedBy:  w.UpdatedBy,
		UpdatedAt:  parseTime(w.UpdatedAt),
	}
	for _, v := range w.Variables {
		out.Variables = append(out.Variables, workspaceScriptVariableFromAPI(v))
	}
	return out
}

func workspaceScriptVariableFromAPI(v rdeapi.WorkspaceScriptVariable) WorkspaceScriptVariable {
	out := WorkspaceScriptVariable{Key: v.Key, Value: v.Value, Visibility: VariableVisibilityPlain}
	switch {
	case v.Protected:
		// Write-only: never surface a value, even if one came back.
		out.Visibility = VariableVisibilityProtected
		out.Value = ""
		out.ValueStored = v.ValueStored
	case v.IsSecret:
		out.Visibility = VariableVisibilitySecret
	}
	return out
}
