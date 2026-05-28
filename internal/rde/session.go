package rde

import (
	"context"
	"time"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

// Session is the CLI-facing session record. JSON tags define the stable
// `--output json` shape. Field set kept minimal per the RDE plan — expand
// additively as users ask for more.
type Session struct {
	ID                          string                   `json:"id"`
	Name                        string                   `json:"name"`
	Description                 string                   `json:"description,omitempty"`
	Status                      string                   `json:"status,omitempty"`
	TemplateID                  string                   `json:"template_id,omitempty"`
	TemplateName                string                   `json:"template_name,omitempty"`
	TemplateDeleted             bool                     `json:"template_deleted,omitempty"`
	TemplateOutdated            bool                     `json:"template_outdated,omitempty"`
	TemplateSnapshot            *SessionTemplateSnapshot `json:"template_snapshot,omitempty"`
	AgentSessionStatus          string                   `json:"agent_session_status,omitempty"`
	AgentSessionStatusUpdatedAt *time.Time               `json:"agent_session_status_updated_at,omitempty"`
	AIEnabled                   bool                     `json:"ai_enabled,omitempty"`
	AIConfigured                bool                     `json:"ai_configured,omitempty"`
	AIPrompt                    string                   `json:"ai_prompt,omitempty"`
	AutoTerminateMinutes        int                      `json:"auto_terminate_minutes,omitempty"`
	AutoTerminateAt             *time.Time               `json:"auto_terminate_at,omitempty"`
	SSHAddress                  string                   `json:"ssh_address,omitempty"`
	// SSHPassword is the ephemeral SSH password issued for this session.
	// Excluded from --output json with json:"-" — secrets shouldn't leak
	// into the stable contract. The field is consumed internally by
	// `rde session exec` for the SSH dial.
	SSHPassword          string     `json:"-"`
	SSHConnectionOpen    bool       `json:"ssh_connection_open,omitempty"`
	VNCAddress           string     `json:"vnc_address,omitempty"`
	VNCUsername          string     `json:"vnc_username,omitempty"`
	PersistentDiskStatus string     `json:"persistent_disk_status,omitempty"`
	CreatedAt            *time.Time `json:"created_at,omitempty"`
	UpdatedAt            *time.Time `json:"updated_at,omitempty"`
}

// SessionTemplateSnapshot is the CLI shape of the template config captured
// at session creation. Mirrors the wire type but with snake_case tags and
// without the masked secret bag.
type SessionTemplateSnapshot struct {
	TemplateName     string          `json:"template_name,omitempty"`
	Image            string          `json:"image,omitempty"`
	MachineType      string          `json:"machine_type,omitempty"`
	WorkingDirectory string          `json:"working_directory,omitempty"`
	HasStartupScript bool            `json:"has_startup_script,omitempty"`
	HasWarmupScript  bool            `json:"has_warmup_script,omitempty"`
	SessionInputs    []SnapshotInput `json:"session_inputs,omitempty"`
	FeatureFlags     []SnapshotFlag  `json:"feature_flags,omitempty"`
	WorkspaceLinks   []SnapshotLink  `json:"workspace_links,omitempty"`
	UpdatedAt        *time.Time      `json:"updated_at,omitempty"`
}

// SnapshotInput is a captured session-input value.
type SnapshotInput struct {
	Key            string `json:"key"`
	Value          string `json:"value,omitempty"`
	IsSecret       bool   `json:"is_secret,omitempty"`
	ExposeAsEnvVar bool   `json:"expose_as_env_var,omitempty"`
}

// SnapshotFlag is a captured feature-flag state.
type SnapshotFlag struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled,omitempty"`
}

// SnapshotLink is a captured workspace link.
type SnapshotLink struct {
	Label      string `json:"label,omitempty"`
	FolderPath string `json:"folder_path,omitempty"`
	SortOrder  int    `json:"sort_order,omitempty"`
}

// SessionInputValue mirrors the wire type for create-session.
type SessionInputValue struct {
	Key          string
	Value        string
	IsSecret     bool
	SavedInputID string
}

// AutoMappedInput records keys auto-filled from saved inputs during create.
type AutoMappedInput struct {
	SessionInputKey string `json:"session_input_key"`
	SavedInputID    string `json:"saved_input_id"`
}

// CreateSessionRequest is the CLI-side request shape. AutoTerminateMinutes
// is a pointer so "not provided" stays distinguishable from "0 = disable".
type CreateSessionRequest struct {
	Name                    string
	Description             string
	TemplateID              string
	SessionInputs           []SessionInputValue
	EnabledFeatureFlagNames []string
	Cluster                 string
	AIPrompt                string
	AutoTerminateMinutes    *int
	MapSavedToSessionInputs bool
}

// UpdateSessionRequest carries optional patch fields. Pointer fields
// preserve unset semantics.
type UpdateSessionRequest struct {
	Name                 *string
	Description          *string
	AutoTerminateMinutes *int
}

// CreateSessionResult is what the create endpoint returns: the new session
// plus any inputs that were auto-filled from saved inputs.
type CreateSessionResult struct {
	Session          Session           `json:"session"`
	AutoMappedInputs []AutoMappedInput `json:"auto_mapped_inputs,omitempty"`
}

// ListSessions returns every session in the workspace for the caller.
func (s *Service) ListSessions(ctx context.Context, workspaceID string) ([]Session, error) {
	if s.client == nil {
		return nil, errClient()
	}
	wire, err := s.client.ListSessions(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]Session, 0, len(wire))
	for _, w := range wire {
		out = append(out, sessionFromAPI(w))
	}
	return out, nil
}

// GetSession returns a session by ID.
func (s *Service) GetSession(ctx context.Context, workspaceID, sessionID string) (Session, error) {
	if s.client == nil {
		return Session{}, errClient()
	}
	w, err := s.client.GetSession(ctx, workspaceID, sessionID)
	if err != nil {
		return Session{}, err
	}
	return sessionFromAPI(w), nil
}

// CreateSession creates a session from a template.
func (s *Service) CreateSession(ctx context.Context, workspaceID string, req CreateSessionRequest) (CreateSessionResult, error) {
	if s.client == nil {
		return CreateSessionResult{}, errClient()
	}
	wireInputs := make([]rdeapi.SessionInputValue, 0, len(req.SessionInputs))
	for _, i := range req.SessionInputs {
		wireInputs = append(wireInputs, rdeapi.SessionInputValue{
			Key:          i.Key,
			Value:        i.Value,
			IsSecret:     i.IsSecret,
			SavedInputID: i.SavedInputID,
		})
	}
	session, mapped, err := s.client.CreateSession(ctx, workspaceID, rdeapi.CreateSessionRequest{
		Name:                    req.Name,
		Description:             req.Description,
		TemplateID:              req.TemplateID,
		SessionInputs:           wireInputs,
		EnabledFeatureFlagNames: req.EnabledFeatureFlagNames,
		Cluster:                 req.Cluster,
		AIPrompt:                req.AIPrompt,
		AutoTerminateMinutes:    req.AutoTerminateMinutes,
		MapSavedToSessionInputs: req.MapSavedToSessionInputs,
	})
	if err != nil {
		return CreateSessionResult{}, err
	}
	res := CreateSessionResult{Session: sessionFromAPI(session)}
	for _, m := range mapped {
		res.AutoMappedInputs = append(res.AutoMappedInputs, AutoMappedInput{
			SessionInputKey: m.SessionInputKey,
			SavedInputID:    m.SavedInputID,
		})
	}
	return res, nil
}

// UpdateSession patches name, description, or auto-terminate minutes.
func (s *Service) UpdateSession(ctx context.Context, workspaceID, sessionID string, req UpdateSessionRequest) (Session, error) {
	if s.client == nil {
		return Session{}, errClient()
	}
	w, err := s.client.UpdateSession(ctx, workspaceID, sessionID, rdeapi.UpdateSessionRequest{
		Name:                 req.Name,
		Description:          req.Description,
		AutoTerminateMinutes: req.AutoTerminateMinutes,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromAPI(w), nil
}

// RestoreSession restores a terminated session by re-provisioning its VM
// from the persistent disk. The session re-enters the STARTING state and
// (assuming no failures) reaches RUNNING again.
func (s *Service) RestoreSession(ctx context.Context, workspaceID, sessionID string) (Session, error) {
	if s.client == nil {
		return Session{}, errClient()
	}
	w, err := s.client.RestoreSession(ctx, workspaceID, sessionID)
	if err != nil {
		return Session{}, err
	}
	return sessionFromAPI(w), nil
}

// TerminateSession terminates a running session (preserves the session for
// later restart; the VM goes away).
func (s *Service) TerminateSession(ctx context.Context, workspaceID, sessionID string) (Session, error) {
	if s.client == nil {
		return Session{}, errClient()
	}
	w, err := s.client.TerminateSession(ctx, workspaceID, sessionID)
	if err != nil {
		return Session{}, err
	}
	return sessionFromAPI(w), nil
}

// TemplateConfig is the CLI shape of the template config on either side of
// a session's template diff.
type TemplateConfig struct {
	TemplateName      string                   `json:"template_name,omitempty"`
	Image             string                   `json:"image,omitempty"`
	MachineType       string                   `json:"machine_type,omitempty"`
	WorkingDirectory  string                   `json:"working_directory,omitempty"`
	StartupScript     string                   `json:"startup_script,omitempty"`
	WarmupScript      string                   `json:"warmup_script,omitempty"`
	SessionInputs     []TemplateConfigInput    `json:"session_inputs,omitempty"`
	FeatureFlags      []TemplateConfigFlag     `json:"feature_flags,omitempty"`
	TemplateVariables []TemplateConfigVariable `json:"template_variables,omitempty"`
	WorkspaceLinks    []SnapshotLink           `json:"workspace_links,omitempty"`
	UpdatedAt         *time.Time               `json:"updated_at,omitempty"`
}

// TemplateConfigInput mirrors the diff-endpoint session-input definition.
type TemplateConfigInput struct {
	Key            string `json:"key"`
	Description    string `json:"description,omitempty"`
	Required       bool   `json:"required,omitempty"`
	DefaultValue   string `json:"default_value,omitempty"`
	ExposeAsEnvVar bool   `json:"expose_as_env_var,omitempty"`
	IsSecret       bool   `json:"is_secret,omitempty"`
}

// TemplateConfigFlag carries default-enabled state alongside name/description.
type TemplateConfigFlag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled,omitempty"`
}

// TemplateConfigVariable is variable metadata (values stripped server-side).
type TemplateConfigVariable struct {
	Key            string `json:"key"`
	IsSecret       bool   `json:"is_secret,omitempty"`
	ExposeAsEnvVar bool   `json:"expose_as_env_var,omitempty"`
}

// SessionTemplateDiff is the CLI shape of /template-diff.
type SessionTemplateDiff struct {
	Snapshot            *TemplateConfig `json:"snapshot,omitempty"`
	Current             *TemplateConfig `json:"current,omitempty"`
	ChangedVariableKeys []string        `json:"changed_variable_keys,omitempty"`
}

// DiffSessionTemplate returns the snapshot-vs-current template diff for
// a session. Current is nil when the template was deleted.
func (s *Service) DiffSessionTemplate(ctx context.Context, workspaceID, sessionID string) (SessionTemplateDiff, error) {
	if s.client == nil {
		return SessionTemplateDiff{}, errClient()
	}
	w, err := s.client.CompareSessionTemplate(ctx, workspaceID, sessionID)
	if err != nil {
		return SessionTemplateDiff{}, err
	}
	out := SessionTemplateDiff{ChangedVariableKeys: w.ChangedVariableKeys}
	if w.Snapshot != nil {
		c := templateConfigFromAPI(*w.Snapshot)
		out.Snapshot = &c
	}
	if w.Current != nil {
		c := templateConfigFromAPI(*w.Current)
		out.Current = &c
	}
	return out, nil
}

func templateConfigFromAPI(w rdeapi.TemplateConfig) TemplateConfig {
	out := TemplateConfig{
		TemplateName:     w.TemplateName,
		Image:            w.Image,
		MachineType:      w.MachineType,
		WorkingDirectory: w.WorkingDirectory,
		StartupScript:    w.StartupScript,
		WarmupScript:     w.WarmupScript,
		UpdatedAt:        parseTime(w.UpdatedAt),
	}
	for _, i := range w.SessionInputs {
		out.SessionInputs = append(out.SessionInputs, TemplateConfigInput{
			Key:            i.Key,
			Description:    i.Description,
			Required:       i.Required,
			DefaultValue:   i.DefaultValue,
			ExposeAsEnvVar: i.ExposeAsEnvVar,
			IsSecret:       i.IsSecret,
		})
	}
	for _, f := range w.FeatureFlags {
		out.FeatureFlags = append(out.FeatureFlags, TemplateConfigFlag{
			Name:        f.Name,
			Description: f.Description,
			Enabled:     f.Enabled,
		})
	}
	for _, v := range w.TemplateVariables {
		out.TemplateVariables = append(out.TemplateVariables, TemplateConfigVariable{
			Key:            v.Key,
			IsSecret:       v.IsSecret,
			ExposeAsEnvVar: v.ExposeAsEnvVar,
		})
	}
	for _, l := range w.WorkspaceLinks {
		out.WorkspaceLinks = append(out.WorkspaceLinks, SnapshotLink{
			Label:      l.Label,
			FolderPath: l.FolderPath,
			SortOrder:  l.SortOrder,
		})
	}
	return out
}

// WaitForReady polls GetSession until the session leaves the provisioning
// states ("" / "pending" / "starting") and returns the resulting Session.
// The caller decides whether the returned status counts as success.
// Returns context.Canceled when ctx is cancelled.
func (s *Service) WaitForReady(ctx context.Context, workspaceID, sessionID string, interval time.Duration) (Session, error) {
	if s.client == nil {
		return Session{}, errClient()
	}
	if interval <= 0 {
		interval = 3 * time.Second
	}
	for {
		sess, err := s.GetSession(ctx, workspaceID, sessionID)
		if err != nil {
			return Session{}, err
		}
		switch sess.Status {
		case "", "pending", "starting":
			// still provisioning — keep polling
		default:
			return sess, nil
		}
		select {
		case <-ctx.Done():
			return Session{}, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// WaitForTerminated polls GetSession until the session leaves the
// transitional teardown states ("terminating" / "draining") and returns the
// resulting Session — normally "terminated" (or "failed"). The caller decides
// whether the final status is acceptable. Returns context.Canceled when ctx
// is cancelled.
//
// This is the teardown companion to WaitForReady: a bare TerminateSession
// returns while the session is still "terminating", so a
// 'terminate && delete' pipeline races the backend — delete rejects any
// session that isn't yet "terminated" or "failed". Waiting here closes
// that gap.
func (s *Service) WaitForTerminated(ctx context.Context, workspaceID, sessionID string, interval time.Duration) (Session, error) {
	if s.client == nil {
		return Session{}, errClient()
	}
	if interval <= 0 {
		interval = 3 * time.Second
	}
	for {
		sess, err := s.GetSession(ctx, workspaceID, sessionID)
		if err != nil {
			return Session{}, err
		}
		switch sess.Status {
		case "terminating", "draining":
			// still tearing down — keep polling
		default:
			return sess, nil
		}
		select {
		case <-ctx.Done():
			return Session{}, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// DeleteSession permanently removes a session.
func (s *Service) DeleteSession(ctx context.Context, workspaceID, sessionID string) error {
	if s.client == nil {
		return errClient()
	}
	return s.client.DeleteSession(ctx, workspaceID, sessionID)
}

// DeleteTerminatedSessions removes every terminated session and returns
// the count of sessions actually deleted.
func (s *Service) DeleteTerminatedSessions(ctx context.Context, workspaceID string) (int, error) {
	if s.client == nil {
		return 0, errClient()
	}
	return s.client.DeleteTerminatedSessions(ctx, workspaceID)
}

func sessionFromAPI(w rdeapi.Session) Session {
	out := Session{
		ID:                   w.ID,
		Name:                 w.Name,
		Description:          w.Description,
		Status:               statusFromAPI(w.Status),
		TemplateID:           w.TemplateID,
		TemplateDeleted:      w.TemplateDeleted,
		TemplateOutdated:     w.TemplateOutdated,
		AgentSessionStatus:   agentStatusFromAPI(w.AgentSessionStatus),
		AIEnabled:            w.AIEnabled,
		AIConfigured:         w.AIConfigured,
		AIPrompt:             w.AIPrompt,
		AutoTerminateMinutes: w.AutoTerminateMinutes,
		SSHAddress:           w.SSHAddress,
		SSHPassword:          w.SSHPassword,
		SSHConnectionOpen:    w.SSHConnectionOpen,
		VNCAddress:           w.VNCAddress,
		VNCUsername:          w.VNCUsername,
		PersistentDiskStatus: diskStatusFromAPI(w.PersistentDiskStatus),
	}
	out.AgentSessionStatusUpdatedAt = parseTime(w.AgentSessionStatusUpdatedAt)
	out.AutoTerminateAt = parseTime(w.AutoTerminateAt)
	out.CreatedAt = parseTime(w.CreatedAt)
	out.UpdatedAt = parseTime(w.UpdatedAt)
	if w.TemplateSnapshot != nil {
		snap := snapshotFromAPI(*w.TemplateSnapshot)
		out.TemplateSnapshot = &snap
		out.TemplateName = snap.TemplateName
	}
	return out
}

func snapshotFromAPI(w rdeapi.SessionTemplateSnapshot) SessionTemplateSnapshot {
	out := SessionTemplateSnapshot{
		TemplateName:     w.TemplateName,
		Image:            w.Image,
		MachineType:      w.MachineType,
		WorkingDirectory: w.WorkingDirectory,
		HasStartupScript: w.HasStartupScript,
		HasWarmupScript:  w.HasWarmupScript,
		UpdatedAt:        parseTime(w.UpdatedAt),
	}
	for _, i := range w.SessionInputs {
		out.SessionInputs = append(out.SessionInputs, SnapshotInput{
			Key:            i.Key,
			Value:          i.Value,
			IsSecret:       i.IsSecret,
			ExposeAsEnvVar: i.ExposeAsEnvVar,
		})
	}
	for _, f := range w.FeatureFlags {
		out.FeatureFlags = append(out.FeatureFlags, SnapshotFlag{
			Name:    f.Name,
			Enabled: f.Enabled,
		})
	}
	for _, l := range w.WorkspaceLinks {
		out.WorkspaceLinks = append(out.WorkspaceLinks, SnapshotLink{
			Label:      l.Label,
			FolderPath: l.FolderPath,
			SortOrder:  l.SortOrder,
		})
	}
	return out
}
