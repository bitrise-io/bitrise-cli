package rde

import (
	"context"
	"fmt"
	"strings"
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
	SSHPassword       string `json:"-"`
	SSHConnectionOpen bool   `json:"ssh_connection_open,omitempty"`
	VNCAddress        string `json:"vnc_address,omitempty"`
	VNCUsername       string `json:"vnc_username,omitempty"`
	// VNCPassword is the ephemeral VNC password issued for this session.
	// Same handling as SSHPassword: excluded from --output json so the
	// stable contract doesn't leak secrets. Surfaced only through the
	// opt-in `rde session vnc` and `rde session open-vnc` commands.
	VNCPassword          string            `json:"-"`
	PersistentDiskStatus string            `json:"persistent_disk_status,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
	// OwnerType says who owns the session: "user" (the creating user; the
	// default) or "workspace" (the workspace itself — visible to every
	// member of the workspace). Passed through verbatim so future owner
	// kinds reach callers without a CLI change.
	OwnerType string `json:"owner_type,omitempty"`
	// OwnerID is the owner's identifier, typed by OwnerType: the owning
	// user's ID for "user", the workspace slug for "workspace".
	OwnerID   string     `json:"owner_id,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	// Device is the session's virtual device (iOS simulator / Android
	// emulator) and its readiness; nil when the session has no device.
	// Status "running" does NOT mean the device is usable — read
	// Device.State.
	Device *SessionDevice `json:"device,omitempty"`
	// WarmPoolID is the warm pool this session belongs to; empty when
	// none. WarmState is its relation to that pool — WarmStateClaimed or
	// WarmStateCold for a session created with `--warm-pool`, "warming" /
	// "ready" for pool inventory. Passed through verbatim so future states
	// reach callers without a CLI change. Empty when the session has no
	// pool.
	WarmPoolID string `json:"warm_pool_id,omitempty"`
	WarmState  string `json:"warm_state,omitempty"`
	// Lifecycle is the session's lifecycle-hook state; nil on a session
	// that runs the legacy warmup/startup scripts. Status "running" means
	// the machine is up, NOT that its setup finished — read
	// Lifecycle.Phase.
	Lifecycle *SessionLifecycle `json:"lifecycle,omitempty"`
}

// SessionLifecycle is a session's lifecycle-hook state: the hook-aware
// phase derived from its runs, and the runs themselves, oldest first.
type SessionLifecycle struct {
	// Mode is "hooks" for a session that runs lifecycle hooks.
	Mode string `json:"mode,omitempty"`
	// Phase is the hook-aware label next to the machine status, lowercased
	// without the LIFECYCLE_PHASE_ prefix: warming / ready_to_claim (warm
	// pool inventory), setting_up (On create running), personalizing (the
	// system setup step or On claim running), restoring, ready (every
	// queued hook done), setup_failed / personalization_failed /
	// restore_failed (the last run failed or was interrupted). Passed
	// through forward-compatibly: an unknown phase reaches callers too.
	Phase string `json:"phase,omitempty"`
	// Runs are every hook run so far, oldest first.
	Runs []HookRun `json:"runs,omitempty"`
	// HasOnClaimHook / HasOnRestoreHook say which optional hooks the
	// session's creation-time template copy has.
	HasOnClaimHook   bool `json:"has_on_claim_hook,omitempty"`
	HasOnRestoreHook bool `json:"has_on_restore_hook,omitempty"`
}

// HookRun is one run of a lifecycle hook on a session.
type HookRun struct {
	ID string `json:"id,omitempty"`
	// Hook: "on_create", "on_claim", "on_restore", or the platform's own
	// "personalize" step (the platform's system setup queued before On claim).
	Hook string `json:"hook"`
	// Attempt numbers the hook's runs on this session from 1.
	Attempt int `json:"attempt,omitempty"`
	// State, lowercased without the HOOK_RUN_STATE_ prefix: pending,
	// running, succeeded (the script exited with code 0; background
	// processes it started may keep running), failed (a non-zero exit, or
	// RDE ended it because On create did not finish within its limit),
	// interrupted (the machine was stopped mid-run), skipped (an earlier run
	// failed).
	State string `json:"state,omitempty"`
	// ExitCode of the script, once it exited. HookExitCodeOnCreateLimit (124)
	// means RDE ended the run because On create did not finish within its
	// limit.
	ExitCode int `json:"exit_code,omitempty"`
	// TriggeredBy: "create", "claim", "restore", "run_again" or "rebuild".
	TriggeredBy string `json:"triggered_by,omitempty"`
	// RanIn: "pool" (inside a warm pool, before any claim) or "session".
	RanIn      string     `json:"ran_in,omitempty"`
	QueuedAt   *time.Time `json:"queued_at,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// HookExitCodeOnCreateLimit is the HookRun.ExitCode of an On create run RDE
// ended because it did not finish within its limit (2 hours): the script
// has to exit — long-running work belongs in the background.
const HookExitCodeOnCreateLimit = 124

// ErrSessionSettingUp is the backend's answer to any interactive action
// (exec, ssh, upload, download, screenshot, the terminal) while the
// session's On create run is pending or running — FailedPrecondition with
// this message. The CLI's exec / ssh dial the machine directly, so they
// apply the same gate client-side (see sshTargetForSession) and say the
// same thing.
const ErrSessionSettingUp = "Session is setting up: On create is still running. Access opens when it finishes."

// lifecycleFromAPI maps the wire lifecycle, normalizing the enums the way
// statusFromAPI does; nil stays nil.
func lifecycleFromAPI(w *rdeapi.SessionLifecycle) *SessionLifecycle {
	if w == nil {
		return nil
	}
	out := &SessionLifecycle{
		Mode:             w.Mode,
		Phase:            enumFromAPI("LIFECYCLE_PHASE_", w.Phase),
		HasOnClaimHook:   w.HasOnClaimHook,
		HasOnRestoreHook: w.HasOnRestoreHook,
	}
	for _, r := range w.Runs {
		out.Runs = append(out.Runs, hookRunFromAPI(r))
	}
	return out
}

func hookRunFromAPI(r rdeapi.HookRun) HookRun {
	return HookRun{
		ID:          r.ID,
		Hook:        r.Hook,
		Attempt:     r.Attempt,
		State:       enumFromAPI("HOOK_RUN_STATE_", r.State),
		ExitCode:    r.ExitCode,
		TriggeredBy: r.TriggeredBy,
		RanIn:       r.RanIn,
		QueuedAt:    parseTime(r.QueuedAt),
		StartedAt:   parseTime(r.StartedAt),
		FinishedAt:  parseTime(r.FinishedAt),
	}
}

// Hook names of a session's lifecycle runs: the template's three hooks and
// the platform's own system setup step. RunSessionHook accepts all but
// HookOnCreate (a fresh On create needs a fresh machine: RebuildSession).
const (
	HookOnCreate    = "on_create"
	HookOnClaim     = "on_claim"
	HookOnRestore   = "on_restore"
	HookPersonalize = "personalize"
)

// ErrOnCreateNotRunnable is the backend's answer (FailedPrecondition) to
// running On create again on a session: On create runs once per machine, so
// a fresh attempt needs a fresh machine (RebuildSession). RunSessionHook
// refuses it client-side with the same words.
const ErrOnCreateNotRunnable = "On create cannot be run again on this machine. Use Rebuild to run it on a fresh one."

// RunSessionHook queues one hook (or the platform's system setup step) to
// run again on a session that runs lifecycle hooks and returns the queued
// run (state pending, triggered_by "run_again"). On create is not accepted
// (ErrOnCreateNotRunnable; use RebuildSession). The backend refuses the
// rest while a run is pending or running, on warm pool inventory and on a
// session that never ran the given hook.
func (s *Service) RunSessionHook(ctx context.Context, workspaceID, sessionID, hook string) (HookRun, error) {
	if s.client == nil {
		return HookRun{}, errClient()
	}
	switch hook {
	case HookOnClaim, HookOnRestore, HookPersonalize:
	case HookOnCreate:
		return HookRun{}, fmt.Errorf("%s", ErrOnCreateNotRunnable)
	default:
		return HookRun{}, fmt.Errorf("invalid hook %q (must be %s, %s or %s)", hook, HookOnClaim, HookOnRestore, HookPersonalize)
	}
	r, err := s.client.RunSessionHook(ctx, workspaceID, sessionID, hook)
	if err != nil {
		return HookRun{}, err
	}
	return hookRunFromAPI(r), nil
}

// RebuildSession discards the session's machine (VM and disk) and boots a
// fresh one for the same session; its lifecycle runs again from On create.
func (s *Service) RebuildSession(ctx context.Context, workspaceID, sessionID string) (Session, error) {
	if s.client == nil {
		return Session{}, errClient()
	}
	w, err := s.client.RebuildSession(ctx, workspaceID, sessionID)
	if err != nil {
		return Session{}, err
	}
	return sessionFromAPI(w), nil
}

// DeviceSpec describes a virtual device in the preview-link vocabulary:
// Platform "ios" or "android"; the rest optional (platform defaults).
type DeviceSpec struct {
	Platform    string `json:"platform"`
	DeviceModel string `json:"device_model,omitempty"`
	OSVersion   string `json:"os_version,omitempty"`
	SystemImage string `json:"system_image,omitempty"`
	RAMMb       uint32 `json:"ram_mb,omitempty"`
	Cores       uint32 `json:"cores,omitempty"`
	ColdBoot    bool   `json:"cold_boot,omitempty"`
}

// PlatformLabel is the human name of the device kind: "iOS simulator",
// "Android emulator", or "device" for an unknown/unset platform.
func (d DeviceSpec) PlatformLabel() string {
	switch d.Platform {
	case "ios":
		return "iOS simulator"
	case "android":
		return "Android emulator"
	}
	return "device"
}

// Summary renders the device as "platform · model · version" for human
// output, omitting the parts that aren't set. iOS carries the OS version;
// Android identifies the OS by its system image package instead, so that
// stands in for the version to keep the detail complete on both platforms.
func (d DeviceSpec) Summary() string {
	what := d.PlatformLabel()
	if d.DeviceModel != "" {
		what += " · " + d.DeviceModel
	}
	version := d.OSVersion
	if version == "" {
		version = d.SystemImage
	}
	if version != "" {
		what += " · " + version
	}
	return what
}

// DeviceArtifact is an app build to install once the device is ready.
type DeviceArtifact struct {
	URL         string `json:"url"`
	AppName     string `json:"app_name,omitempty"`
	BuildNumber string `json:"build_number,omitempty"`
	CommitSHA   string `json:"commit_sha,omitempty"`
}

// SessionDevice mirrors the API's Session.device. State/InstallStatus are
// normalized like Status: "booting" / "ready" / "failed" / "" and
// "pending" / "running" / "ok" / "failed" / "".
type SessionDevice struct {
	Spec          *DeviceSpec `json:"spec,omitempty"`
	State         string      `json:"state,omitempty"`
	DeviceNotes   string      `json:"device_notes,omitempty"`
	InstallStatus string      `json:"install_status,omitempty"`
	InstallReason string      `json:"install_reason,omitempty"`
	AppName       string      `json:"app_name,omitempty"`
	BuildNumber   string      `json:"build_number,omitempty"`
	CommitSHA     string      `json:"commit_sha,omitempty"`
	// PageURL is the device's page in the RDE web UI, for a browser where
	// the user is logged in to Bitrise (not a shareable link). Empty on
	// unclaimed warm pool inventory; set once the session is claimed.
	PageURL string `json:"page_url,omitempty"`
}

// deviceStateFromAPI maps PREVIEW_DEVICE_STATE_* to a short lowercase word
// ("booting" / "ready" / "failed"; "" for unspecified) the same way
// statusFromAPI does for sessions: the prefix is stripped and the rest
// lowercased, so a state added after this code was written still reaches
// callers as a recognizable word instead of collapsing to "".
func deviceStateFromAPI(v string) string {
	return enumFromAPI("PREVIEW_DEVICE_STATE_", v)
}

// installStatusFromAPI maps PREVIEW_INSTALL_STATUS_* to a short word
// ("pending" / "running" / "ok" / "failed"; "" for unspecified), forward
// compatible like deviceStateFromAPI.
func installStatusFromAPI(v string) string {
	return enumFromAPI("PREVIEW_INSTALL_STATUS_", v)
}

// enumFromAPI strips prefix from a proto enum name and lowercases the rest;
// the empty string and the *_UNSPECIFIED zero value map to "".
func enumFromAPI(prefix, v string) string {
	if v == "" {
		return ""
	}
	v = strings.TrimPrefix(v, prefix)
	if v == "UNSPECIFIED" {
		return ""
	}
	return strings.ToLower(v)
}

func deviceFromAPI(w *rdeapi.SessionDevice) *SessionDevice {
	if w == nil {
		return nil
	}
	out := &SessionDevice{
		State:         deviceStateFromAPI(w.State),
		DeviceNotes:   w.DeviceNotes,
		InstallStatus: installStatusFromAPI(w.InstallStatus),
		InstallReason: w.InstallReason,
		AppName:       w.AppName,
		BuildNumber:   w.BuildNumber,
		CommitSHA:     w.CommitSHA,
		PageURL:       w.PageURL,
	}
	out.Spec = deviceSpecFromAPI(w.Spec)
	return out
}

func deviceSpecFromAPI(w *rdeapi.DeviceSpec) *DeviceSpec {
	if w == nil {
		return nil
	}
	return &DeviceSpec{
		Platform: w.Platform, DeviceModel: w.DeviceModel, OSVersion: w.OSVersion,
		SystemImage: w.SystemImage, RAMMb: w.RAMMb, Cores: w.Cores, ColdBoot: w.ColdBoot,
	}
}

// Resumable reports whether `rde claude` can resume this session: a running
// session can be reattached, and a terminated/stopped/failed one can be
// restored as long as its persistent disk is still available. Any other
// (transitional) state is reported resumable optimistically — it's an
// in-flight status that should settle into one of the above. Whether the
// session can actually be resumed at that moment is decided by the resume flow,
// which reattaches a running session, restores a terminated/stopped/failed one,
// and asks the user to retry shortly for anything still in flight.
func (s Session) Resumable() bool {
	switch s.Status {
	case "terminated", "stopped", "failed":
		return s.PersistentDiskStatus != DiskStatusUnavailable
	default:
		return true
	}
}

// SessionTemplateSnapshot is the CLI shape of the template config captured
// at session creation. Mirrors the wire type but with snake_case tags and
// without the masked secret bag.
type SessionTemplateSnapshot struct {
	TemplateName     string `json:"template_name,omitempty"`
	StackID          string `json:"stack_id,omitempty"`
	MachineType      string `json:"machine_type,omitempty"`
	WorkingDirectory string `json:"working_directory,omitempty"`
	HasStartupScript bool   `json:"has_startup_script,omitempty"`
	HasWarmupScript  bool   `json:"has_warmup_script,omitempty"`
	// LifecycleMode is "hooks" when the session runs lifecycle hooks (a
	// template with hooks, or a legacy template mapped at creation); empty
	// on the legacy path. The Has*Hook flags say which hooks the
	// creation-time template copy has.
	LifecycleMode    string          `json:"lifecycle_mode,omitempty"`
	HasOnCreateHook  bool            `json:"has_on_create_hook,omitempty"`
	HasOnClaimHook   bool            `json:"has_on_claim_hook,omitempty"`
	HasOnRestoreHook bool            `json:"has_on_restore_hook,omitempty"`
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
	// Hook is the scope the value belongs to: "shared" (also the meaning
	// of empty), "on_create", "on_claim" or "on_restore" (see Scopes).
	Hook string `json:"hook,omitempty"`
	// SuppliedByPool: the pool supplied this value and the claimer never
	// receives it (On create scope on a session a pool handed out); Value
	// is empty.
	SuppliedByPool bool `json:"supplied_by_pool,omitempty"`
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
	// Hook is the scope the value is for (see Scopes); empty applies it to
	// every scope that defines the key, and a scoped value wins over an
	// unscoped one for its scope.
	Hook string
}

// AutoMappedInput records keys auto-filled from saved inputs during create.
type AutoMappedInput struct {
	SessionInputKey string `json:"session_input_key"`
	SavedInputID    string `json:"saved_input_id"`
}

// CreateSessionRequest is the CLI-side request shape. AutoTerminateMinutes
// is a pointer so "not provided" stays distinguishable from "0 = disable".
// Labels is arbitrary key=value metadata attached to the session; the
// backend validates it (entry count, key and value charset/length,
// reserved "bitrise.io/" key prefix).
type CreateSessionRequest struct {
	Name                    string
	Description             string
	TemplateID              string
	StackID                 string
	MachineType             string
	SessionInputs           []SessionInputValue
	EnabledFeatureFlagNames []string
	Cluster                 string
	AIPrompt                string
	AutoTerminateMinutes    *int
	MapSavedToSessionInputs bool
	Labels                  map[string]string
	// DeviceSpec boots a virtual device with the session (optional).
	DeviceSpec *DeviceSpec
	// Artifact is an app build to install once the device is ready
	// (optional; requires DeviceSpec).
	Artifact *DeviceArtifact
	// NoDevice creates the session without the device its template
	// declares (invalid together with DeviceSpec).
	NoDevice bool
	// OwnerType is SessionOwnerUser (personal; the backend default when
	// empty) or SessionOwnerWorkspace (owned by the workspace itself).
	OwnerType string
	// WarmPoolID claims a session from a warm pool instead of building one
	// from this request. The pool fixes the configuration: TemplateID,
	// StackID, MachineType, SessionInputs, EnabledFeatureFlagNames,
	// Cluster, AIPrompt, MapSavedToSessionInputs, DeviceSpec and NoDevice
	// must be empty (the backend rejects them); OwnerType must be empty or
	// the pool's. Name, Description, Labels, AutoTerminateMinutes and
	// Artifact apply to the claimed session.
	WarmPoolID string
}

// Session owner kinds accepted by CreateSessionRequest.OwnerType (and the
// `session create --owner` flag). Empty means the backend default: a
// personal session for a PAT, a workspace-owned one for a Workspace API
// Token (which cannot create personal sessions at all).
const (
	SessionOwnerUser      = "user"
	SessionOwnerWorkspace = "workspace"
)

// UpdateSessionRequest carries optional patch fields. Pointer fields
// preserve unset semantics. Labels upserts into the session's existing
// labels; RemoveLabels deletes keys (a key in both is removed — the
// backend gives removal precedence).
type UpdateSessionRequest struct {
	Name                 *string
	Description          *string
	AutoTerminateMinutes *int
	Labels               map[string]string
	RemoveLabels         []string
}

// CreateSessionResult is what the create endpoint returns: the new session
// plus any inputs that were auto-filled from saved inputs.
type CreateSessionResult struct {
	Session          Session           `json:"session"`
	AutoMappedInputs []AutoMappedInput `json:"auto_mapped_inputs,omitempty"`
}

// Session list scopes accepted by ListSessions' scope argument (and the
// `session list --scope` flag). Empty means the backend default, which is
// SessionScopeMine.
const (
	SessionScopeMine      = "mine"
	SessionScopeWorkspace = "workspace"
)

// ListSessions returns sessions in the workspace, optionally filtered by
// label selectors ("key=value" exact matches, ANDed) — pass nil for the full
// list. scope selects whose sessions are listed: SessionScopeMine (the
// caller's own; also the backend default when empty) or SessionScopeWorkspace
// (sessions owned by the workspace itself, visible to every member).
func (s *Service) ListSessions(ctx context.Context, workspaceID string, labelSelectors []string, scope string) ([]Session, error) {
	if s.client == nil {
		return nil, errClient()
	}
	wire, err := s.client.ListSessions(ctx, workspaceID, labelSelectors, scope)
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

// ResolveSessionID maps `value` to a session ID. UUID-shaped inputs
// short-circuit (no network call); names trigger a ListSessions call and an
// exact case-insensitive match. Errors clearly when zero or multiple sessions
// match the name, so callers can surface ambiguity to the user.
//
// Session names aren't unique (unlike a UUID), so an ambiguous match is an
// expected outcome — the error lists the candidate IDs so the user can re-run
// with the exact one. Mirrors ResolveTemplateID.
//
// Names resolve against the default scope only (the caller's own sessions);
// workspace-owned sessions are addressed by ID.
func (s *Service) ResolveSessionID(ctx context.Context, workspaceID, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("session is required")
	}
	if looksLikeUUID(value) {
		return value, nil
	}
	sessions, err := s.ListSessions(ctx, workspaceID, nil, "")
	if err != nil {
		return "", fmt.Errorf("list sessions to resolve %q: %w", value, err)
	}
	var matches []Session
	for _, sess := range sessions {
		if equalFold(sess.Name, value) {
			matches = append(matches, sess)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no session named %q in workspace (try 'rde session list')", value)
	case 1:
		return matches[0].ID, nil
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		return "", fmt.Errorf("session name %q is ambiguous (matches %d sessions: %v) — pass a session ID instead", value, len(matches), ids)
	}
}

// CreateSession creates a session. Provide either a TemplateID or, for a
// templateless session, a StackID + MachineType.
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
			Hook:         i.Hook,
		})
	}
	session, mapped, err := s.client.CreateSession(ctx, workspaceID, rdeapi.CreateSessionRequest{
		Name:                    req.Name,
		Description:             req.Description,
		TemplateID:              req.TemplateID,
		StackID:                 req.StackID,
		MachineType:             req.MachineType,
		SessionInputs:           wireInputs,
		EnabledFeatureFlagNames: req.EnabledFeatureFlagNames,
		Cluster:                 req.Cluster,
		AIPrompt:                req.AIPrompt,
		AutoTerminateMinutes:    req.AutoTerminateMinutes,
		MapSavedToSessionInputs: req.MapSavedToSessionInputs,
		Labels:                  req.Labels,
		DeviceSpec:              deviceSpecToAPI(req.DeviceSpec),
		Artifact:                artifactToAPI(req.Artifact),
		NoDevice:                req.NoDevice,
		OwnerType:               req.OwnerType,
		WarmPoolID:              req.WarmPoolID,
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

// UpdateSession patches name, description, auto-terminate minutes, or
// labels (upserts via Labels, removals via RemoveLabels).
func (s *Service) UpdateSession(ctx context.Context, workspaceID, sessionID string, req UpdateSessionRequest) (Session, error) {
	if s.client == nil {
		return Session{}, errClient()
	}
	w, err := s.client.UpdateSession(ctx, workspaceID, sessionID, rdeapi.UpdateSessionRequest{
		Name:                 req.Name,
		Description:          req.Description,
		AutoTerminateMinutes: req.AutoTerminateMinutes,
		Labels:               req.Labels,
		RemoveLabels:         req.RemoveLabels,
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
	TemplateName     string `json:"template_name,omitempty"`
	StackID          string `json:"stack_id,omitempty"`
	MachineType      string `json:"machine_type,omitempty"`
	WorkingDirectory string `json:"working_directory,omitempty"`
	StartupScript    string `json:"startup_script,omitempty"`
	WarmupScript     string `json:"warmup_script,omitempty"`
	// LifecycleMode ("legacy" or "hooks") and Hooks: the session's
	// creation-time hooks on the snapshot side, the template's current
	// hooks on the current side; nil without hooks.
	LifecycleMode     string                   `json:"lifecycle_mode,omitempty"`
	Hooks             *TemplateHooks           `json:"hooks,omitempty"`
	SessionInputs     []TemplateConfigInput    `json:"session_inputs,omitempty"`
	FeatureFlags      []TemplateConfigFlag     `json:"feature_flags,omitempty"`
	TemplateVariables []TemplateConfigVariable `json:"template_variables,omitempty"`
	WorkspaceLinks    []SnapshotLink           `json:"workspace_links,omitempty"`
	DeviceSpec        *DeviceSpec              `json:"device_spec,omitempty"`
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
	Hook           string `json:"hook,omitempty"`
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
	// Hook is the variable's scope (see Scopes).
	Hook string `json:"hook,omitempty"`
	// Protected: write-only (see TemplateVariable.Protected).
	Protected bool `json:"protected,omitempty"`
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
		StackID:          firstNonEmpty(w.StackID, w.Image),
		MachineType:      w.MachineType,
		WorkingDirectory: w.WorkingDirectory,
		StartupScript:    w.StartupScript,
		WarmupScript:     w.WarmupScript,
		LifecycleMode:    w.LifecycleMode,
		Hooks:            hooksFromAPI(w.Hooks),
		DeviceSpec:       deviceSpecFromAPI(w.DeviceSpec),
		UpdatedAt:        parseTime(w.UpdatedAt),
	}
	for _, i := range w.SessionInputs {
		// Mask secret default values at the CLI boundary, same as
		// snapshotFromAPI — the backend may return them in cleartext and
		// the diff is rendered verbatim in --output json.
		def := i.DefaultValue
		if i.IsSecret {
			def = ""
		}
		out.SessionInputs = append(out.SessionInputs, TemplateConfigInput{
			Key:            i.Key,
			Description:    i.Description,
			Required:       i.Required,
			DefaultValue:   def,
			ExposeAsEnvVar: i.ExposeAsEnvVar,
			IsSecret:       i.IsSecret,
			Hook:           i.Hook,
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
			IsSecret:       v.IsSecret || v.Protected,
			ExposeAsEnvVar: v.ExposeAsEnvVar,
			Hook:           v.Hook,
			Protected:      v.Protected,
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
// states ("" / "pending" / "starting" / "unknown") and, on a session that
// runs lifecycle hooks, until its lifecycle phase settles too, then returns
// the resulting Session. "unknown" means the backend can't currently
// determine the machine state; it's expected to settle, so it's treated as
// still provisioning.
//
// On a lifecycle-hooks session "running" arrives as soon as the machine is
// up, BEFORE On create (the migrated warmup) has finished — so a caller that
// stopped there and ran a command would race the setup. The wait therefore
// keeps polling a running hooks session while Lifecycle.Phase is in flight
// (setting_up / personalizing / restoring, or not derived yet) and returns
// once it is "ready" or a failure phase; LifecycleFailure turns the latter
// into an error. A legacy session (no lifecycle) returns at "running" as
// before. The caller decides whether the returned status counts as success.
// Returns context.Canceled when ctx is cancelled.
//
// onPoll, when non-nil, is called with the session's status on every poll
// ("running: setting_up" while a hooks session is settling), so a caller can
// surface the live provisioning state (e.g. in a progress spinner) without
// polling separately. It must not block.
func (s *Service) WaitForReady(ctx context.Context, workspaceID, sessionID string, interval time.Duration, onPoll func(status string)) (Session, error) {
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
		settling := sess.Status == "running" && lifecycleInFlight(sess.Lifecycle)
		if onPoll != nil {
			if settling {
				onPoll(sess.Status + ": " + lifecyclePhaseOrPending(sess.Lifecycle))
			} else {
				onPoll(sess.Status)
			}
		}
		switch {
		case settling:
			// machine up, lifecycle hooks still running — keep polling
		case sess.Status == "", sess.Status == "pending", sess.Status == "starting", sess.Status == "unknown":
			// still provisioning (or state not yet determinable) — keep polling
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

// lifecycleInFlight reports whether a session's lifecycle hooks are still
// running (or not derived yet) on a session that runs them. nil (a legacy
// session) is never in flight. Any phase this code doesn't know — ready, the
// failure phases, pool inventory states, a value added later — counts as
// settled, so a newer backend can't make the wait spin until its timeout.
// OnCreateInFlight reports whether the session's On create run is pending
// or running — the state in which the backend refuses every interactive
// action with ErrSessionSettingUp. It reads the runs, not the phase, because
// warm pool inventory warms under its own phase and a legacy session has no
// lifecycle at all (false).
func OnCreateInFlight(l *SessionLifecycle) bool {
	if l == nil {
		return false
	}
	for _, r := range l.Runs {
		if r.Hook == HookOnCreate && (r.State == "pending" || r.State == "running") {
			return true
		}
	}
	return false
}

func lifecycleInFlight(l *SessionLifecycle) bool {
	// A session that does not run lifecycle hooks has nothing to wait for:
	// the backend sends it a lifecycle block too (mode "legacy", no phase).
	if l == nil || l.Mode != "hooks" {
		return false
	}
	switch l.Phase {
	case "", "setting_up", "personalizing", "restoring":
		return true
	}
	return false
}

func lifecyclePhaseOrPending(l *SessionLifecycle) string {
	if l == nil || l.Phase == "" {
		return "setting up"
	}
	return l.Phase
}

// LifecycleFailure is the error a wait ends with when the session's machine
// is up but its lifecycle hooks did not get it ready: nil on a legacy
// session, on a ready one, or while hooks are still in flight. The message
// names the failed phase and the log stage that holds the output, because
// that is what the caller needs next.
func LifecycleFailure(sess Session) error {
	if sess.Lifecycle == nil {
		return nil
	}
	var what, stage string
	switch sess.Lifecycle.Phase {
	case "setup_failed":
		what, stage = "its On create hook failed", LogStageOnCreate
	case "personalization_failed":
		what, stage = "its system setup or On claim hook failed", LogStageOnClaim
	case "restore_failed":
		what, stage = "its On restore hook failed", LogStageOnRestore
	default:
		return nil
	}
	return fmt.Errorf("session %s is running but %s (lifecycle phase %q); see 'rde session logs %s --stage %s' and 'rde session view %s'",
		sess.ID, what, sess.Lifecycle.Phase, sess.ID, stage, sess.ID)
}

// WaitForSSHReady polls GetSession until the session's SSH endpoint is usable
// — connection open and address + password populated — and returns the
// resulting Session. A "running" status (what WaitForReady waits for) does not
// guarantee SSH is up: the backend issues credentials a few seconds later, so
// callers that want to dial in must wait on this too.
//
// If the session leaves the "running" state while waiting (e.g. it fails or is
// terminated), it returns an error rather than spinning forever. Returns
// context.Canceled when ctx is cancelled.
func (s *Service) WaitForSSHReady(ctx context.Context, workspaceID, sessionID string, interval time.Duration) (Session, error) {
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
		if sess.Status != "running" {
			return Session{}, fmt.Errorf("session is no longer running (status: %q) while waiting for SSH", sess.Status)
		}
		if sess.SSHConnectionOpen && sess.SSHAddress != "" && sess.SSHPassword != "" {
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
// returns while the session is still "terminating". Callers that need the
// shutdown (and disk snapshot) to have completed — e.g. before restoring —
// wait here. Note that DeleteSession no longer needs this: it accepts a
// session in any state and stops the VM itself.
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

// DeleteTerminatedSessions removes the caller's own terminated sessions in
// the workspace (never other members') and returns the count actually
// deleted.
// DeleteTerminatedSessions hard-deletes the terminated sessions in one
// ownership scope: SessionScopeMine (the caller's own; also the backend
// default when empty) or SessionScopeWorkspace (the workspace-owned ones).
func (s *Service) DeleteTerminatedSessions(ctx context.Context, workspaceID, scope string) (int, error) {
	if s.client == nil {
		return 0, errClient()
	}
	return s.client.DeleteTerminatedSessions(ctx, workspaceID, scope)
}

func deviceSpecToAPI(d *DeviceSpec) *rdeapi.DeviceSpec {
	if d == nil {
		return nil
	}
	return &rdeapi.DeviceSpec{
		Platform: d.Platform, DeviceModel: d.DeviceModel, OSVersion: d.OSVersion,
		SystemImage: d.SystemImage, RAMMb: d.RAMMb, Cores: d.Cores, ColdBoot: d.ColdBoot,
	}
}

func artifactToAPI(a *DeviceArtifact) *rdeapi.DeviceArtifact {
	if a == nil {
		return nil
	}
	return &rdeapi.DeviceArtifact{URL: a.URL, AppName: a.AppName, BuildNumber: a.BuildNumber, CommitSHA: a.CommitSHA}
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
		VNCPassword:          w.VNCPassword,
		PersistentDiskStatus: diskStatusFromAPI(w.PersistentDiskStatus),
		Labels:               w.Labels,
		OwnerType:            w.OwnerType,
		OwnerID:              w.OwnerID,
		Device:               deviceFromAPI(w.Device),
		WarmPoolID:           w.WarmPoolID,
		WarmState:            w.WarmState,
		Lifecycle:            lifecycleFromAPI(w.Lifecycle),
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
		StackID:          firstNonEmpty(w.StackID, w.Image),
		MachineType:      w.MachineType,
		WorkingDirectory: w.WorkingDirectory,
		HasStartupScript: w.HasStartupScript,
		HasWarmupScript:  w.HasWarmupScript,
		LifecycleMode:    w.LifecycleMode,
		HasOnCreateHook:  w.HasOnCreateHook,
		HasOnClaimHook:   w.HasOnClaimHook,
		HasOnRestoreHook: w.HasOnRestoreHook,
		UpdatedAt:        parseTime(w.UpdatedAt),
	}
	for _, i := range w.SessionInputs {
		// Mask secret values at the CLI boundary: passing them through would
		// leak the value into stdout, shell history, and log files. Keep the
		// key + is_secret marker so callers can still see what was set.
		// Because the CLI never opts into include_secrets on session reads,
		// the backend already omits these values — this masking is
		// the belt-and-suspenders second line of defense in case the backend
		// default ever changes.
		val := i.Value
		if i.IsSecret {
			val = ""
		}
		out.SessionInputs = append(out.SessionInputs, SnapshotInput{
			Key:            i.Key,
			Value:          val,
			IsSecret:       i.IsSecret,
			ExposeAsEnvVar: i.ExposeAsEnvVar,
			Hook:           i.Hook,
			SuppliedByPool: i.SuppliedByPool,
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
