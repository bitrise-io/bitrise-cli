package session

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newVNCCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "vnc SESSION_ID",
		Short: "Print VNC connection credentials for a session",
		Long: `Print the VNC connection details (address, username, password, and a
ready-to-use vnc:// URL) for a session.

The VNC password is ephemeral and tied to this session. Avoid pasting the
output into chat or sharing it — anyone with the URL can connect to the
session. ` + "`rde session view`" + ` and other commands intentionally hide it.

In human mode the URL is the only thing on stdout, so it's safe to pipe:

  open "$(bitrise-cli rde session vnc SESSION_ID)"

In --output json mode a {address, username, password, url} object is
emitted. Prefer ` + "`rde session open-vnc`" + ` when you just want to launch your
viewer — that hands the URL to the OS without printing the password.`,
		Example: `  bitrise-cli rde session vnc SESSION_ID
  bitrise-cli rde session vnc SESSION_ID --output json
  open "$(bitrise-cli rde session vnc SESSION_ID)"`,
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
			return output.Render(cmd.OutOrStdout(), format, creds, renderVNCCredentials)
		},
	}
	return c
}

func renderVNCCredentials(w io.Writer, creds internalrde.VNCCredentials) error {
	_, err := fmt.Fprintln(w, creds.URL)
	return err
}
