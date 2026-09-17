package rde

import (
	"context"
	"fmt"
	"time"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

// WarmPool is the CLI-facing warm pool record. JSON tags define the stable
// `--output json` shape.
//
// A warm pool is a stored session configuration — template, session input
// values, feature flags, optional stack / machine type / cluster and device
// overrides — plus an owner and a desired count. The backend keeps
// DesiredCount sessions of that configuration booted and idle ("warm
// sessions"); `session create --warm-pool` hands one out instantly, or
// creates one from the configuration when none is available.
type WarmPool struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	WorkspaceID  string `json:"workspace_id,omitempty"`
	TemplateID   string `json:"template_id,omitempty"`
	TemplateName string `json:"template_name,omitempty"`
	// OwnerType is SessionOwnerUser (private to its creator) or
	// SessionOwnerWorkspace (shared with every member); OwnerID is the
	// user ID or the workspace slug accordingly.
	OwnerType      string `json:"owner_type,omitempty"`
	OwnerID        string `json:"owner_id,omitempty"`
	CreatedByEmail string `json:"created_by_email,omitempty"`
	// DesiredCount is how many warm sessions the backend keeps booted.
	// 0 drains the pool but keeps it usable as a configuration preset —
	// always emitted, since 0 is a meaningful value.
	DesiredCount int `json:"desired_count"`
	// SessionInputs are the values the warm sessions are created with.
	// Secret values are masked (empty Value, IsSecret true).
	SessionInputs           []WarmPoolInput `json:"session_inputs,omitempty"`
	EnabledFeatureFlagNames []string        `json:"enabled_feature_flag_names,omitempty"`
	// StackID, MachineType and Cluster override the template's; empty
	// means the template's value.
	StackID     string `json:"stack_id,omitempty"`
	MachineType string `json:"machine_type,omitempty"`
	Cluster     string `json:"cluster,omitempty"`
	// DeviceSpec overrides the template's declared device; nil boots the
	// template's device as declared (or none). NoDevice skips it.
	DeviceSpec *DeviceSpec `json:"device_spec,omitempty"`
	NoDevice   bool        `json:"no_device,omitempty"`
	// Status is the pool's live state; nil only if the backend omitted it.
	Status    *WarmPoolStatus `json:"status,omitempty"`
	CreatedAt *time.Time      `json:"created_at,omitempty"`
	UpdatedAt *time.Time      `json:"updated_at,omitempty"`
}

// WarmPoolInput is a stored session input value: either a plain or secret
// Value, or a reference to a saved input by ID.
type WarmPoolInput struct {
	Key          string `json:"key"`
	Value        string `json:"value,omitempty"`
	IsSecret     bool   `json:"is_secret,omitempty"`
	SavedInputID string `json:"saved_input_id,omitempty"`
}

// WarmPoolStatus is the live state of a warm pool. The counters are always
// emitted: 0 ready is the number a caller acts on.
type WarmPoolStatus struct {
	// Ready and Warming count the current inventory.
	Ready   int `json:"ready"`
	Warming int `json:"warming"`
	// ClaimedTotal and ColdTotal are lifetime counters. A growing
	// ColdTotal means the desired count is too low for the demand.
	ClaimedTotal int `json:"claimed_total"`
	ColdTotal    int `json:"cold_total"`
	// LastError is the last machine-creation failure; ConfigError says
	// why the stored configuration no longer builds (the pool creates
	// nothing until it is fixed). Both empty when healthy.
	LastError   string `json:"last_error,omitempty"`
	ConfigError string `json:"config_error,omitempty"`
	// PausedUntil is until when the backend backs off after repeated
	// failures; nil when active.
	PausedUntil *time.Time `json:"paused_until,omitempty"`
	// Sessions is the inventory, oldest first — filled by GetWarmPool
	// only, empty on list responses.
	Sessions []WarmPoolSession `json:"sessions,omitempty"`
}

// WarmPoolSession is one warm session in a pool's inventory.
type WarmPoolSession struct {
	SessionID string `json:"session_id"`
	// State is "warming" or "ready".
	State     string     `json:"state,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	// ReadyAt is when the session became ready; nil while warming.
	ReadyAt *time.Time `json:"ready_at,omitempty"`
}

// Warm-state values a session created with `--warm-pool` reports in
// Session.WarmState: WarmStateClaimed when a warm session was handed out,
// WarmStateCold when one had to be created from the pool's configuration.
// Pool inventory sessions report "warming" / "ready".
const (
	WarmStateClaimed = "claimed"
	WarmStateCold    = "cold"
)

// CreateWarmPoolRequest is the CLI-side request shape. Apart from Name,
// OwnerType and DesiredCount these are the CreateSessionRequest fields
// that shape the VM — a pool stores what a session request could send.
type CreateWarmPoolRequest struct {
	Name       string
	TemplateID string
	// OwnerType is SessionOwnerUser (the backend default for a personal
	// token) or SessionOwnerWorkspace (shared; the only option for a
	// Workspace API Token). Empty leaves the backend default.
	OwnerType string
	// DesiredCount may be 0: an inert pool that still serves as a preset.
	DesiredCount            int
	SessionInputs           []SessionInputValue
	MapSavedToSessionInputs bool
	EnabledFeatureFlagNames []string
	StackID                 string
	MachineType             string
	Cluster                 string
	DeviceSpec              *DeviceSpec
	NoDevice                bool
}

// UpdateWarmPoolRequest carries optional patch fields. Pointer scalars
// preserve "unset, leave alone" semantics (an empty string clears an
// override); the list pointers replace the pool's list when non-nil, even
// when empty — which clears it. The device is replaced when DeviceSpec is
// non-nil and removed when ClearDeviceSpec is set. A configuration change
// makes the backend replace the pool's current inventory.
type UpdateWarmPoolRequest struct {
	Name                    *string
	DesiredCount            *int
	SessionInputs           *[]SessionInputValue
	EnabledFeatureFlagNames *[]string
	StackID                 *string
	MachineType             *string
	Cluster                 *string
	DeviceSpec              *DeviceSpec
	ClearDeviceSpec         bool
	NoDevice                *bool
}

// ListWarmPools returns the warm pools visible to the caller — the
// workspace's pools plus the caller's own — optionally narrowed to one
// template (templateID non-empty). all lists every pool in the workspace
// read-only; the backend gates that on the billing-data permission.
func (s *Service) ListWarmPools(ctx context.Context, workspaceID, templateID string, all bool) ([]WarmPool, error) {
	if s.client == nil {
		return nil, errClient()
	}
	wire, err := s.client.ListWarmPools(ctx, workspaceID, templateID, all)
	if err != nil {
		return nil, err
	}
	out := make([]WarmPool, 0, len(wire))
	for _, w := range wire {
		out = append(out, warmPoolFromAPI(w))
	}
	return out, nil
}

// GetWarmPool returns a warm pool by ID, with its live status and inventory.
func (s *Service) GetWarmPool(ctx context.Context, workspaceID, warmPoolID string) (WarmPool, error) {
	if s.client == nil {
		return WarmPool{}, errClient()
	}
	w, err := s.client.GetWarmPool(ctx, workspaceID, warmPoolID)
	if err != nil {
		return WarmPool{}, err
	}
	return warmPoolFromAPI(w), nil
}

// ResolveWarmPoolID maps `value` to a warm pool ID. UUID-shaped inputs
// short-circuit (no network call); names trigger a ListWarmPools call over
// the pools visible to the caller and an exact case-insensitive match.
// Errors clearly when zero or multiple pools match the name. Mirrors
// ResolveTemplateID.
func (s *Service) ResolveWarmPoolID(ctx context.Context, workspaceID, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("warm pool is required")
	}
	if looksLikeUUID(value) {
		return value, nil
	}
	pools, err := s.ListWarmPools(ctx, workspaceID, "", false)
	if err != nil {
		return "", fmt.Errorf("list warm pools to resolve %q: %w", value, err)
	}
	var matches []WarmPool
	for _, p := range pools {
		if equalFold(p.Name, value) {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no warm pool named %q in workspace (try 'rde warm-pool list')", value)
	case 1:
		return matches[0].ID, nil
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		return "", fmt.Errorf("warm pool name %q is ambiguous (matches %d pools: %v) — pass a warm pool ID instead", value, len(matches), ids)
	}
}

// CreateWarmPool creates a warm pool.
func (s *Service) CreateWarmPool(ctx context.Context, workspaceID string, req CreateWarmPoolRequest) (WarmPool, error) {
	if s.client == nil {
		return WarmPool{}, errClient()
	}
	if req.Name == "" {
		return WarmPool{}, fmt.Errorf("name is required")
	}
	if req.TemplateID == "" {
		return WarmPool{}, fmt.Errorf("template is required")
	}
	if req.DesiredCount < 0 {
		return WarmPool{}, fmt.Errorf("desired count must not be negative")
	}
	w, err := s.client.CreateWarmPool(ctx, workspaceID, rdeapi.CreateWarmPoolRequest{
		Name:                    req.Name,
		TemplateID:              req.TemplateID,
		OwnerType:               req.OwnerType,
		DesiredCount:            req.DesiredCount,
		SessionInputs:           sessionInputsToAPI(req.SessionInputs),
		MapSavedToSessionInputs: req.MapSavedToSessionInputs,
		EnabledFeatureFlagNames: req.EnabledFeatureFlagNames,
		StackID:                 req.StackID,
		MachineType:             req.MachineType,
		Cluster:                 req.Cluster,
		DeviceSpec:              deviceSpecToAPI(req.DeviceSpec),
		NoDevice:                req.NoDevice,
	})
	if err != nil {
		return WarmPool{}, err
	}
	return warmPoolFromAPI(w), nil
}

// UpdateWarmPool patches a warm pool. Pointer scalars are sent only when
// non-nil; the list pointers trigger their update switch when non-nil
// (even when empty — which clears the list); the device is replaced when
// DeviceSpec is non-nil and removed when ClearDeviceSpec is set.
func (s *Service) UpdateWarmPool(ctx context.Context, workspaceID, warmPoolID string, req UpdateWarmPoolRequest) (WarmPool, error) {
	if s.client == nil {
		return WarmPool{}, errClient()
	}
	if req.DesiredCount != nil && *req.DesiredCount < 0 {
		return WarmPool{}, fmt.Errorf("desired count must not be negative")
	}
	wire := rdeapi.UpdateWarmPoolRequest{
		Name:         req.Name,
		DesiredCount: req.DesiredCount,
		StackID:      req.StackID,
		MachineType:  req.MachineType,
		Cluster:      req.Cluster,
		NoDevice:     req.NoDevice,
	}
	if req.SessionInputs != nil {
		wire.SessionInputs = sessionInputsToAPI(*req.SessionInputs)
		wire.UpdateSessionInputs = true
	}
	if req.EnabledFeatureFlagNames != nil {
		wire.EnabledFeatureFlagNames = *req.EnabledFeatureFlagNames
		wire.UpdateEnabledFeatureFlagNames = true
	}
	if req.DeviceSpec != nil || req.ClearDeviceSpec {
		wire.DeviceSpec = deviceSpecToAPI(req.DeviceSpec)
		wire.UpdateDeviceSpec = true
	}
	w, err := s.client.UpdateWarmPool(ctx, workspaceID, warmPoolID, wire)
	if err != nil {
		return WarmPool{}, err
	}
	return warmPoolFromAPI(w), nil
}

// DeleteWarmPool deletes a warm pool. Its warm sessions are terminated by
// the backend; sessions already claimed are untouched.
func (s *Service) DeleteWarmPool(ctx context.Context, workspaceID, warmPoolID string) error {
	if s.client == nil {
		return errClient()
	}
	return s.client.DeleteWarmPool(ctx, workspaceID, warmPoolID)
}

// sessionInputsToAPI converts session input values to the wire shape. Nil
// in, nil out, so an absent list stays absent on the wire.
func sessionInputsToAPI(in []SessionInputValue) []rdeapi.SessionInputValue {
	if in == nil {
		return nil
	}
	out := make([]rdeapi.SessionInputValue, 0, len(in))
	for _, i := range in {
		out = append(out, rdeapi.SessionInputValue{
			Key:          i.Key,
			Value:        i.Value,
			IsSecret:     i.IsSecret,
			SavedInputID: i.SavedInputID,
		})
	}
	return out
}

func warmPoolFromAPI(w rdeapi.WarmPool) WarmPool {
	out := WarmPool{
		ID:                      w.ID,
		Name:                    w.Name,
		WorkspaceID:             w.WorkspaceID,
		TemplateID:              w.TemplateID,
		TemplateName:            w.TemplateName,
		OwnerType:               w.OwnerType,
		OwnerID:                 w.OwnerID,
		CreatedByEmail:          w.CreatedByEmail,
		DesiredCount:            w.DesiredCount,
		EnabledFeatureFlagNames: w.EnabledFeatureFlagNames,
		StackID:                 w.StackID,
		MachineType:             w.MachineType,
		Cluster:                 w.Cluster,
		DeviceSpec:              deviceSpecFromAPI(w.DeviceSpec),
		NoDevice:                w.NoDevice,
		CreatedAt:               parseTime(w.CreatedAt),
		UpdatedAt:               parseTime(w.UpdatedAt),
	}
	for _, i := range w.SessionInputs {
		// Mask secret values at the CLI boundary, like snapshotFromAPI:
		// the backend already redacts them, this is the second line of
		// defense in case that default ever changes.
		val := i.Value
		if i.IsSecret {
			val = ""
		}
		out.SessionInputs = append(out.SessionInputs, WarmPoolInput{
			Key:          i.Key,
			Value:        val,
			IsSecret:     i.IsSecret,
			SavedInputID: i.SavedInputID,
		})
	}
	if w.Status != nil {
		st := warmPoolStatusFromAPI(*w.Status)
		out.Status = &st
	}
	return out
}

func warmPoolStatusFromAPI(w rdeapi.WarmPoolStatus) WarmPoolStatus {
	out := WarmPoolStatus{
		Ready:        w.Ready,
		Warming:      w.Warming,
		ClaimedTotal: w.ClaimedTotal,
		ColdTotal:    w.ColdTotal,
		LastError:    w.LastError,
		ConfigError:  w.ConfigError,
		PausedUntil:  parseTime(w.PausedUntil),
	}
	for _, sess := range w.Sessions {
		out.Sessions = append(out.Sessions, WarmPoolSession{
			SessionID: sess.SessionID,
			State:     sess.State,
			CreatedAt: parseTime(sess.CreatedAt),
			ReadyAt:   parseTime(sess.ReadyAt),
		})
	}
	return out
}
