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

// newHostBridge builds the host bridge for the session along with the allowlist
// of local actions the in-session Claude may trigger. The closures are built
// here, in the cmd layer, so internal/rde never depends on the cmd packages.
//
// The only action is open-vnc: it fetches the session's VNC credentials and
// opens a viewer on the user's local machine pointed at the session's desktop.
// The VNC password is read locally and handed straight to the OS handler — it
// never travels to the remote.
//
// Debug is left nil on purpose: the bridge runs concurrently with the
// full-screen Claude TUI, which holds the terminal in raw mode, so any
// diagnostic write would corrupt the display (the metadata monitor is silent
// for the same reason).
func newHostBridge(svc *internalrde.Service, workspaceID, sessionID string) *internalrde.HostBridge {
	return &internalrde.HostBridge{
		Service:     svc,
		WorkspaceID: workspaceID,
		SessionID:   sessionID,
		Actions: map[string]internalrde.HostAction{
			internalrde.ActionOpenVNC: {Handle: func(ctx context.Context, _ *http.Request) (any, error) {
				creds, err := svc.GetSessionVNC(ctx, workspaceID, sessionID)
				if err != nil {
					return nil, err
				}
				if err := openVNCURL(ctx, creds.URL); err != nil {
					return nil, fmt.Errorf("open VNC URL: %w", err)
				}
				return map[string]any{"opened": true, "address": creds.Address}, nil
			}},
		},
	}
}
