package claude

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

// openVNCURL hands a vnc:// URL to the OS viewer. A package var so tests can
// assert what the open-vnc action would launch without opening anything.
var openVNCURL = cmdutil.OpenVNCURL

// newHostBridge builds the host bridge for the session with the given allowlist
// of local actions. A pure constructor — the action set (and the decision of
// which actions apply to this session) is made by the caller, so internal/rde
// never depends on the cmd packages.
func newHostBridge(svc *internalrde.Service, workspaceID, sessionID string, actions map[string]internalrde.HostAction) *internalrde.HostBridge {
	return &internalrde.HostBridge{
		Service:     svc,
		WorkspaceID: workspaceID,
		SessionID:   sessionID,
		Actions:     actions,
		// Debug is left nil on purpose: the bridge runs concurrently with the
		// full-screen Claude TUI, which holds the terminal in raw mode, so any
		// diagnostic write would corrupt the display (the metadata monitor is
		// silent for the same reason).
	}
}

// localHostActions returns the host actions applicable to this session.
//
// open-vnc is offered only when the session exposes a VNC endpoint — currently
// macOS sessions; Linux sessions have no VNC. When it isn't offered the bridge
// has no actions, so the caller skips it and the skill that describes open-vnc
// is never written — the in-session Claude is never told about a capability the
// session can't fulfill.
func localHostActions(ctx context.Context, svc *internalrde.Service, workspaceID, sessionID string) map[string]internalrde.HostAction {
	actions := map[string]internalrde.HostAction{}
	if hasVNC, err := svc.SessionExposesVNC(ctx, workspaceID, sessionID); err == nil && hasVNC {
		actions[internalrde.ActionOpenVNC] = internalrde.HostAction{Handle: openVNCAction(svc, workspaceID, sessionID)}
	}
	return actions
}

// openVNCAction fetches the session's VNC credentials and opens a viewer on the
// user's local machine pointed at the session's desktop. The VNC password is
// read locally and handed straight to the OS handler — it never travels to the
// remote.
func openVNCAction(svc *internalrde.Service, workspaceID, sessionID string) func(context.Context, *http.Request) (any, error) {
	return func(ctx context.Context, _ *http.Request) (any, error) {
		creds, err := svc.GetSessionVNC(ctx, workspaceID, sessionID)
		if err != nil {
			return nil, err
		}
		if err := openVNCURL(ctx, creds.URL); err != nil {
			return nil, fmt.Errorf("open VNC URL: %w", err)
		}
		return map[string]any{"opened": true, "address": creds.Address}, nil
	}
}
