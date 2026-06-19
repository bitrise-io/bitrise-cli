package session

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

// openVNCResult is the --output json shape of `session open-vnc`. The
// password is intentionally omitted — `open-vnc` hands the URL to the OS
// handler, so there's no reason to also print it.
type openVNCResult struct {
	Opened   bool   `json:"opened"`
	Address  string `json:"address"`
	Username string `json:"username,omitempty"`
}

// urlOpener spawns the platform-appropriate URL handler. Overridable in
// tests so we can assert what we'd run without launching anything.
var urlOpener = cmdutil.OpenVNCURL

func newOpenVNCCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "open-vnc SESSION_ID",
		Short: "Open a session's VNC endpoint in the OS-default viewer",
		Long: `Hand the session's VNC URL to the operating system's default URL handler:

  - macOS:    /usr/bin/open
  - Linux:    xdg-open (must be installed; install x11-utils or similar)
  - Windows:  cmd /c start

The OS launches whatever app is registered for vnc:// (Screen Sharing on
macOS by default; Remmina/Vinagre on Linux; a third-party client on Windows).

The URL contains the ephemeral VNC password as a userinfo component. The
URL is passed as an argv element to the OS handler, so it is briefly
visible to other processes on the machine that can read this process's
argv (e.g. ` + "`ps`" + `). On a single-user dev machine this is usually fine;
on a shared host, prefer ` + "`rde session vnc`" + ` and paste the URL into your
viewer manually.`,
		Example: `  bitrise-cli rde session open-vnc SESSION_ID
  bitrise-cli rde session open-vnc SESSION_ID --output json`,
		Args: cmdutil.RequireArgs("SESSION_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			svc := internalrde.NewService(client)
			sessionID, err := svc.ResolveSessionID(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			creds, err := svc.GetSessionVNC(cmd.Context(), workspaceID, sessionID)
			if err != nil {
				return err
			}
			if err := urlOpener(cmd.Context(), creds.URL); err != nil {
				return fmt.Errorf("open VNC URL: %w", err)
			}
			res := openVNCResult{Opened: true, Address: creds.Address, Username: creds.Username}
			if format == output.JSON {
				return output.Render(cmd.OutOrStdout(), format, res, nil)
			}
			if !cmdutil.IsQuiet(cmd) {
				_, err := fmt.Fprintf(cmd.ErrOrStderr(), "Opened VNC viewer for %s\n", creds.Address)
				return err
			}
			return nil
		},
	}
	return c
}
