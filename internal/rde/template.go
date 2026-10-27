package rde

import (
	"context"
	"fmt"
	"time"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

// Template is the CLI-facing template record.
type Template struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	StackID          string `json:"stack_id,omitempty"`
	MachineType      string `json:"machine_type,omitempty"`
	WorkingDirectory string `json:"working_directory,omitempty"`
	StartupScript    string `json:"startup_script,omitempty"`
	WarmupScript     string `json:"warmup_script,omitempty"`
	// LifecycleMode says how the template's scripts run: "legacy"
	// (startup_script / warmup_script) or "hooks" (Hooks, run on the booted
	// machine). Passed through verbatim so future modes reach callers
	// without a CLI change.
	LifecycleMode string `json:"lifecycle_mode,omitempty"`
	// Hooks are the lifecycle hook scripts of a "hooks" template; nil on a
	// legacy template.
	Hooks             *TemplateHooks     `json:"hooks,omitempty"`
	CreatedByEmail    string             `json:"created_by_email,omitempty"`
	WorkspaceID       string             `json:"workspace_id,omitempty"`
	TemplateVariables []TemplateVariable `json:"template_variables,omitempty"`
	SessionInputs     []SessionInputDef  `json:"session_inputs,omitempty"`
	FeatureFlags      []FeatureFlag      `json:"feature_flags,omitempty"`
	WorkspaceLinks    []WorkspaceLink    `json:"workspace_links,omitempty"`
	// DeviceSpec is the device sessions created from the template boot
	// unless overridden at create time; nil when none is declared.
	DeviceSpec *DeviceSpec `json:"device_spec,omitempty"`
	CreatedAt  *time.Time  `json:"created_at,omitempty"`
	UpdatedAt  *time.Time  `json:"updated_at,omitempty"`
}

// TemplateHooks are a template's lifecycle hook scripts (bash), each run
// on the booted machine as the session user. On create runs once when the
// session is created (on a warm pool: when the pool warms the machine, with
// the pool's shared inputs); On claim runs on every claim, when the session
// is assigned to its owner; On restore runs every time a stopped session
// is restored. Empty means "no such hook". A hook is done when its script
// exits (0 = done, non-zero = failed); long-running work is started in the
// background so the script can exit. RDE ends an On create that has not
// exited within 20 minutes (exit code 124), and refuses every interactive
// action on the session while On create is pending or running.
type TemplateHooks struct {
	OnCreate  string `json:"on_create,omitempty"`
	OnClaim   string `json:"on_claim,omitempty"`
	OnRestore string `json:"on_restore,omitempty"`
}

// Scopes of a template's values. Every variable and session input belongs
// to one scope — its `hook`: ScopeShared reaches every hook, the others reach
// that hook only (and need a template with lifecycle hooks). A hook receives
// the shared values plus its own, nothing else; its own value wins over a
// shared value of the same key. Keys are unique within a scope and the same
// key may exist on several scopes. Legacy templates, and specs without a
// hook key, keep every value shared — which is exactly what they had.
const (
	ScopeShared    = "shared"
	ScopeOnCreate  = HookOnCreate
	ScopeOnClaim   = HookOnClaim
	ScopeOnRestore = HookOnRestore
)

// Scopes lists the scopes in run order: shared values are applied with On
// create, then each hook's own; later steps override earlier ones.
var Scopes = []string{ScopeShared, ScopeOnCreate, ScopeOnClaim, ScopeOnRestore}

// NormalizeScope maps a wire `hook` to a scope: empty means shared.
func NormalizeScope(hook string) string {
	if hook == "" {
		return ScopeShared
	}
	return hook
}

// ScopeLabel renders a scope the way the product names it ("Shared", "On
// create", …); an unknown value renders as-is so a scope added after this
// code was written still shows up.
func ScopeLabel(hook string) string {
	switch NormalizeScope(hook) {
	case ScopeShared:
		return "Shared"
	case ScopeOnCreate:
		return "On create"
	case ScopeOnClaim:
		return "On claim"
	case ScopeOnRestore:
		return "On restore"
	}
	return hook
}

// TemplateVariable is a baked-in variable of a template.
type TemplateVariable struct {
	Key      string `json:"key"`
	Value    string `json:"value,omitempty"`
	IsSecret bool   `json:"is_secret,omitempty"`
	// ExposeAsEnvVar ("Also in terminal"): hooks always receive the value;
	// this additionally writes it to the session's shell profile when its
	// scope's step runs. Refused on a protected variable.
	ExposeAsEnvVar bool `json:"expose_as_env_var,omitempty"`
	// Hook is the variable's scope (see Scopes): empty or "shared", or
	// "on_create" / "on_claim" / "on_restore" on a template with hooks.
	Hook string `json:"hook,omitempty"`
	// Protected: secret and write-only — the API never returns the value,
	// an empty value on update keeps the stored one. Implies IsSecret.
	Protected bool `json:"protected,omitempty"`
}

// SessionInputDef is an input definition on a template.
type SessionInputDef struct {
	Key            string `json:"key"`
	Description    string `json:"description,omitempty"`
	Required       bool   `json:"required,omitempty"`
	DefaultValue   string `json:"default_value,omitempty"`
	ExposeAsEnvVar bool   `json:"expose_as_env_var,omitempty"`
	// Hook is the input's scope (see Scopes): empty or "shared" (supplied
	// when the session or the pool is created), "on_create" (supplied with
	// the pool or the direct create), "on_claim" / "on_restore" (supplied
	// by whoever claims, or at a direct create).
	Hook string `json:"hook,omitempty"`
}

// ScopeKeys is one scope's keys, for grouped listings.
type ScopeKeys struct {
	Scope string
	Keys  []string
}

// MissingRequiredInputs returns the template's required inputs that
// `supplied` does not cover, grouped by scope in run order — what a direct
// create still has to give (a direct create needs every required input of
// every scope). An unscoped supplied value (no Hook) covers every scope
// that defines its key, so one value (or one saved input) fills the same
// key on several scopes; a scoped one covers its scope only. Callers skip
// the check when the backend fills inputs itself
// (map_saved_to_session_inputs).
func MissingRequiredInputs(t Template, supplied []SessionInputValue) []ScopeKeys {
	have := make(map[string]bool, len(supplied))
	scoped := make(map[string]bool, len(supplied))
	for _, v := range supplied {
		if v.Hook == "" {
			have[v.Key] = true
		} else {
			scoped[NormalizeScope(v.Hook)+"\x00"+v.Key] = true
		}
	}
	byScope := map[string][]string{}
	var order []string
	for _, in := range t.SessionInputs {
		scope := NormalizeScope(in.Hook)
		if !in.Required || have[in.Key] || scoped[scope+"\x00"+in.Key] {
			continue
		}
		if _, seen := byScope[scope]; !seen {
			order = append(order, scope)
		}
		byScope[scope] = append(byScope[scope], in.Key)
	}
	var out []ScopeKeys
	for _, scope := range Scopes {
		if keys, ok := byScope[scope]; ok {
			out = append(out, ScopeKeys{Scope: scope, Keys: keys})
			delete(byScope, scope)
		}
	}
	// A scope this code does not know goes last, in the order seen.
	for _, scope := range order {
		if keys, ok := byScope[scope]; ok {
			out = append(out, ScopeKeys{Scope: scope, Keys: keys})
		}
	}
	return out
}

// FeatureFlag is a feature flag defined by a template.
type FeatureFlag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// WorkspaceLink is an IDE folder shortcut bundled with a template.
type WorkspaceLink struct {
	Label      string `json:"label,omitempty"`
	FolderPath string `json:"folder_path,omitempty"`
	SortOrder  int    `json:"sort_order,omitempty"`
}

// TemplateSpec is the editable shape of a template — used for create and
// update payloads. Pointer fields preserve "unset, leave alone" semantics
// on update; slices replace the existing list when non-nil. The JSON tags
// match the snake_case shape `template view --output json` emits, so the
// canonical workflow is view → edit → update.
type TemplateSpec struct {
	Name             *string `json:"name,omitempty"`
	Description      *string `json:"description,omitempty"`
	StackID          *string `json:"stack_id,omitempty"`
	MachineType      *string `json:"machine_type,omitempty"`
	WorkingDirectory *string `json:"working_directory,omitempty"`
	StartupScript    *string `json:"startup_script,omitempty"`
	WarmupScript     *string `json:"warmup_script,omitempty"`

	// Hooks are the lifecycle hooks (On create / On claim / On restore).
	// Nil = don't touch on update (legacy scripts on create); non-nil =
	// replace the template's hooks as a whole, which also turns a legacy
	// template into a hooks template. A template has either hooks or the
	// legacy scripts, never both.
	Hooks *TemplateHooks `json:"hooks,omitempty"`

	// Nil = don't touch on update; non-nil = replace the server's list with
	// these values (even if empty). On create, nil/empty is equivalent.
	TemplateVariables *[]TemplateVariableSpec `json:"template_variables,omitempty"`
	SessionInputs     *[]SessionInputSpec     `json:"session_inputs,omitempty"`
	FeatureFlags      *[]FeatureFlagSpec      `json:"feature_flags,omitempty"`
	WorkspaceLinks    *[]WorkspaceLinkSpec    `json:"workspace_links,omitempty"`

	// DeviceSpec declares the device sessions created from the template
	// boot by default. Nil = don't touch on update (or no device on
	// create); non-nil = set it. To remove a template's device on update,
	// set ClearDeviceSpec instead — encoding/json can't tell a JSON null
	// from an absent key, so clearing is a flag rather than a spec field.
	DeviceSpec      *DeviceSpec `json:"device_spec,omitempty"`
	ClearDeviceSpec bool        `json:"-"`
}

// TemplateVariableSpec is the editable shape of a variable. A spec without
// a hook key is a shared variable (legacy specs keep working).
type TemplateVariableSpec struct {
	Key            string `json:"key"`
	Value          string `json:"value,omitempty"`
	IsSecret       bool   `json:"is_secret,omitempty"`
	ExposeAsEnvVar bool   `json:"expose_as_env_var,omitempty"`
	// Hook: see TemplateVariable.Hook. A per-hook scope needs a template
	// with lifecycle hooks.
	Hook string `json:"hook,omitempty"`
	// Protected: see TemplateVariable.Protected. An empty Value on update
	// keeps the stored one, so the view → edit → update round-trip keeps
	// write-only values.
	Protected bool `json:"protected,omitempty"`
}

// SessionInputSpec is the editable shape of a session input definition. A
// spec without a hook key is a shared input (legacy specs keep working).
type SessionInputSpec struct {
	Key            string `json:"key"`
	Description    string `json:"description,omitempty"`
	Required       bool   `json:"required,omitempty"`
	DefaultValue   string `json:"default_value,omitempty"`
	ExposeAsEnvVar bool   `json:"expose_as_env_var,omitempty"`
	// Hook: see SessionInputDef.Hook. A per-hook scope needs a template
	// with lifecycle hooks.
	Hook string `json:"hook,omitempty"`
}

// FeatureFlagSpec is the editable shape of a feature flag.
type FeatureFlagSpec struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// WorkspaceLinkSpec is the editable shape of a workspace link.
type WorkspaceLinkSpec struct {
	Label           string `json:"label,omitempty"`
	FolderPath      string `json:"folder_path,omitempty"`
	FeatureFlagName string `json:"feature_flag_name,omitempty"`
}

// CreateTemplate creates a new template from spec.
func (s *Service) CreateTemplate(ctx context.Context, workspaceID string, spec TemplateSpec) (Template, error) {
	if s.client == nil {
		return Template{}, errClient()
	}
	if spec.Name == nil || *spec.Name == "" {
		return Template{}, fmt.Errorf("name is required")
	}
	if spec.StackID == nil || *spec.StackID == "" {
		return Template{}, fmt.Errorf("stack_id is required")
	}
	if spec.MachineType == nil || *spec.MachineType == "" {
		return Template{}, fmt.Errorf("machine_type is required")
	}
	req := rdeapi.CreateTemplateRequest{
		Name:             *spec.Name,
		StackID:          *spec.StackID,
		MachineType:      *spec.MachineType,
		Description:      deref(spec.Description),
		WorkingDirectory: deref(spec.WorkingDirectory),
		StartupScript:    deref(spec.StartupScript),
		WarmupScript:     deref(spec.WarmupScript),
		Hooks:            hooksToAPI(spec.Hooks),
		DeviceSpec:       deviceSpecToAPI(spec.DeviceSpec),
	}
	if spec.TemplateVariables != nil {
		req.TemplateVariables = toWireVariables(*spec.TemplateVariables)
	}
	if spec.SessionInputs != nil {
		req.SessionInputs = toWireSessionInputs(*spec.SessionInputs)
	}
	if spec.FeatureFlags != nil {
		req.FeatureFlags = toWireFeatureFlags(*spec.FeatureFlags)
	}
	if spec.WorkspaceLinks != nil {
		req.WorkspaceLinks = toWireWorkspaceLinks(*spec.WorkspaceLinks)
	}
	t, err := s.client.CreateTemplate(ctx, workspaceID, req)
	if err != nil {
		return Template{}, err
	}
	return templateFromAPI(t), nil
}

// UpdateTemplate patches an existing template. Pointer scalars are sent
// only when non-nil; arrays trigger their corresponding updateXxx flag
// when non-nil (even when empty — which clears the existing list). The
// device is replaced when DeviceSpec is non-nil and removed when
// ClearDeviceSpec is set; otherwise it is left alone.
func (s *Service) UpdateTemplate(ctx context.Context, workspaceID, templateID string, spec TemplateSpec) (Template, error) {
	if s.client == nil {
		return Template{}, errClient()
	}
	req := rdeapi.UpdateTemplateRequest{
		Name:             spec.Name,
		Description:      spec.Description,
		StackID:          spec.StackID,
		MachineType:      spec.MachineType,
		WorkingDirectory: spec.WorkingDirectory,
		StartupScript:    spec.StartupScript,
		WarmupScript:     spec.WarmupScript,
	}
	if spec.TemplateVariables != nil {
		req.TemplateVariables = toWireVariables(*spec.TemplateVariables)
		req.UpdateTemplateVariables = true
	}
	if spec.SessionInputs != nil {
		req.SessionInputs = toWireSessionInputs(*spec.SessionInputs)
		req.UpdateSessionInputs = true
	}
	if spec.FeatureFlags != nil {
		req.FeatureFlags = toWireFeatureFlags(*spec.FeatureFlags)
		req.UpdateFeatureFlags = true
	}
	if spec.WorkspaceLinks != nil {
		req.WorkspaceLinks = toWireWorkspaceLinks(*spec.WorkspaceLinks)
		req.UpdateWorkspaceLinks = true
	}
	if spec.DeviceSpec != nil || spec.ClearDeviceSpec {
		req.DeviceSpec = deviceSpecToAPI(spec.DeviceSpec)
		req.UpdateDeviceSpec = true
	}
	if spec.Hooks != nil {
		req.Hooks = hooksToAPI(spec.Hooks)
		req.UpdateHooks = true
	}
	t, err := s.client.UpdateTemplate(ctx, workspaceID, templateID, req)
	if err != nil {
		return Template{}, err
	}
	return templateFromAPI(t), nil
}

// DeleteTemplate removes a template.
func (s *Service) DeleteTemplate(ctx context.Context, workspaceID, templateID string) error {
	if s.client == nil {
		return errClient()
	}
	return s.client.DeleteTemplate(ctx, workspaceID, templateID)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func toWireVariables(in []TemplateVariableSpec) []rdeapi.TemplateVariableCreate {
	out := make([]rdeapi.TemplateVariableCreate, 0, len(in))
	for _, v := range in {
		out = append(out, rdeapi.TemplateVariableCreate{
			Key:            v.Key,
			Value:          v.Value,
			IsSecret:       v.IsSecret,
			ExposeAsEnvVar: v.ExposeAsEnvVar,
			Hook:           v.Hook,
			Protected:      v.Protected,
		})
	}
	return out
}

func toWireSessionInputs(in []SessionInputSpec) []rdeapi.SessionInputCreate {
	out := make([]rdeapi.SessionInputCreate, 0, len(in))
	for _, i := range in {
		out = append(out, rdeapi.SessionInputCreate{
			Key:            i.Key,
			Description:    i.Description,
			Required:       i.Required,
			DefaultValue:   i.DefaultValue,
			ExposeAsEnvVar: i.ExposeAsEnvVar,
			Hook:           i.Hook,
		})
	}
	return out
}

func hooksToAPI(h *TemplateHooks) *rdeapi.TemplateHooks {
	if h == nil {
		return nil
	}
	return &rdeapi.TemplateHooks{OnCreate: h.OnCreate, OnClaim: h.OnClaim, OnRestore: h.OnRestore}
}

func hooksFromAPI(h *rdeapi.TemplateHooks) *TemplateHooks {
	if h == nil {
		return nil
	}
	return &TemplateHooks{OnCreate: h.OnCreate, OnClaim: h.OnClaim, OnRestore: h.OnRestore}
}

// lifecycleModeHooks is the Template.LifecycleMode of a template whose
// scripts are lifecycle hooks.
const lifecycleModeHooks = "hooks"

func toWireFeatureFlags(in []FeatureFlagSpec) []rdeapi.FeatureFlagCreate {
	out := make([]rdeapi.FeatureFlagCreate, 0, len(in))
	for _, f := range in {
		out = append(out, rdeapi.FeatureFlagCreate{
			Name:        f.Name,
			Description: f.Description,
		})
	}
	return out
}

func toWireWorkspaceLinks(in []WorkspaceLinkSpec) []rdeapi.WorkspaceLinkCreate {
	out := make([]rdeapi.WorkspaceLinkCreate, 0, len(in))
	for _, l := range in {
		out = append(out, rdeapi.WorkspaceLinkCreate{
			Label:           l.Label,
			FolderPath:      l.FolderPath,
			FeatureFlagName: l.FeatureFlagName,
		})
	}
	return out
}

// ResolveTemplateID maps `value` to a template ID. UUID-shaped inputs
// short-circuit (no network call); names trigger a ListTemplates call
// and an exact case-insensitive match. Errors clearly when zero or
// multiple templates match the name, so callers can surface ambiguity
// to the user.
func (s *Service) ResolveTemplateID(ctx context.Context, workspaceID, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("template is required")
	}
	if looksLikeUUID(value) {
		return value, nil
	}
	templates, err := s.ListTemplates(ctx, workspaceID)
	if err != nil {
		return "", fmt.Errorf("list templates to resolve %q: %w", value, err)
	}
	var matches []Template
	for _, t := range templates {
		if equalFold(t.Name, value) {
			matches = append(matches, t)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no template named %q in workspace (try 'rde template list')", value)
	case 1:
		return matches[0].ID, nil
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		return "", fmt.Errorf("template name %q is ambiguous (matches %d templates: %v) — pass a template ID instead", value, len(matches), ids)
	}
}

// looksLikeUUID is a syntactic check (8-4-4-4-12 hex). Cheap and good
// enough to distinguish IDs from names — the server validates the real
// format on its side.
func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// ListTemplates returns every template visible to the caller in the workspace.
func (s *Service) ListTemplates(ctx context.Context, workspaceID string) ([]Template, error) {
	if s.client == nil {
		return nil, errClient()
	}
	wire, err := s.client.ListTemplates(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]Template, 0, len(wire))
	for _, w := range wire {
		out = append(out, templateFromAPI(w))
	}
	return out, nil
}

// GetTemplate returns a single template by ID.
func (s *Service) GetTemplate(ctx context.Context, workspaceID, templateID string) (Template, error) {
	if s.client == nil {
		return Template{}, errClient()
	}
	w, err := s.client.GetTemplate(ctx, workspaceID, templateID)
	if err != nil {
		return Template{}, err
	}
	return templateFromAPI(w), nil
}

func templateFromAPI(w rdeapi.Template) Template {
	out := Template{
		ID:               w.ID,
		Name:             w.Name,
		Description:      w.Description,
		StackID:          firstNonEmpty(w.StackID, w.Image),
		MachineType:      w.MachineType,
		WorkingDirectory: w.WorkingDirectory,
		StartupScript:    w.StartupScript,
		WarmupScript:     w.WarmupScript,
		LifecycleMode:    w.LifecycleMode,
		Hooks:            hooksFromAPI(w.Hooks),
		CreatedByEmail:   w.CreatedByEmail,
		WorkspaceID:      w.WorkspaceID,
		DeviceSpec:       deviceSpecFromAPI(w.DeviceSpec),
		CreatedAt:        parseTime(w.CreatedAt),
		UpdatedAt:        parseTime(w.UpdatedAt),
	}
	if w.LifecycleMode == lifecycleModeHooks {
		// A hooks template stores no legacy scripts: the backend reads
		// startup_script back as a generated, read-only equivalent of the
		// hooks, which it refuses on writes. Drop it so the documented
		// round-trip (view -o json → edit → update) sends only what the
		// template really holds — the hooks.
		out.StartupScript, out.WarmupScript = "", ""
	}
	for _, v := range w.TemplateVariables {
		// Mask secret values at the CLI boundary: passing them through
		// would leak the value into stdout, shell history, and log files.
		// Because the CLI never opts into include_secrets on template reads,
		// the backend already omits these values — this masking
		// is the belt-and-suspenders second line of defense.
		// A protected variable is write-only on top: the backend returns
		// no value for it with or without include_secrets.
		val := v.Value
		if v.IsSecret || v.Protected {
			val = ""
		}
		out.TemplateVariables = append(out.TemplateVariables, TemplateVariable{
			Key:            v.Key,
			Value:          val,
			IsSecret:       v.IsSecret || v.Protected,
			ExposeAsEnvVar: v.ExposeAsEnvVar,
			Hook:           v.Hook,
			Protected:      v.Protected,
		})
	}
	for _, i := range w.SessionInputs {
		out.SessionInputs = append(out.SessionInputs, SessionInputDef{
			Key:            i.Key,
			Description:    i.Description,
			Required:       i.Required,
			DefaultValue:   i.DefaultValue,
			ExposeAsEnvVar: i.ExposeAsEnvVar,
			Hook:           i.Hook,
		})
	}
	for _, f := range w.FeatureFlags {
		out.FeatureFlags = append(out.FeatureFlags, FeatureFlag{
			Name:        f.Name,
			Description: f.Description,
		})
	}
	for _, l := range w.WorkspaceLinks {
		out.WorkspaceLinks = append(out.WorkspaceLinks, WorkspaceLink{
			Label:      l.Label,
			FolderPath: l.FolderPath,
			SortOrder:  l.SortOrder,
		})
	}
	return out
}
