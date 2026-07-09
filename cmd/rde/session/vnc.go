package session

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newVNCCmd() *cobra.Command {
	var forwardPort int
	c := &cobra.Command{
		Use:   "vnc SESSION_ID",
		Short: "Print VNC connection details, or forward the endpoint to a local port",
		Long: `Print the VNC connection details (address, host, port, username, password,
and a ready-to-use vnc:// URL) for a session.

The VNC password is ephemeral and tied to this session. Avoid pasting the
output into chat or sharing it — anyone with the URL can connect to the
session. ` + "`rde session view`" + ` and other commands intentionally hide it.

In human mode the URL is the only thing on stdout, so it's safe to pipe:

  open "$(bitrise-cli rde session vnc SESSION_ID)"

In --output json mode a fully-decomposed {address, host, port, username,
password, url} object is emitted — host and port are always discrete fields,
so a caller building its own connection never has to parse the address or URL.

Pass --forward to open an SSH tunnel and expose the session's VNC endpoint on a
local port, then block until Ctrl-C:

  bitrise-cli rde session vnc SESSION_ID --forward        # auto-pick a local port
  bitrise-cli rde session vnc SESSION_ID --forward 5901   # bind localhost:5901

A native VNC client (macOS Screen Sharing, Remmina, …) can then connect to the
printed localhost address. The tunnel rides the same SSH connection the CLI
already uses, so no direct network route to the session is required and no
credentials are embedded in a URL handed to the OS. Prefer ` + "`rde session open-vnc`" + `
when you just want to launch your viewer against a directly-reachable endpoint.`,
		Example: `  bitrise-cli rde session vnc SESSION_ID
  bitrise-cli rde session vnc SESSION_ID --output json
  bitrise-cli rde session vnc SESSION_ID --forward 5901
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

			if cmd.Flags().Changed("forward") {
				return runVNCForward(cmd, svc, workspaceID, sessionID, forwardPort, format)
			}

			creds, err := svc.GetSessionVNC(cmd.Context(), workspaceID, sessionID)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, creds, renderVNCCredentials)
		},
	}
	c.Flags().IntVar(&forwardPort, "forward", 0,
		"forward the session's VNC endpoint to this local port and block until Ctrl-C; omit the value to auto-pick a free port")
	// Allow a bare `--forward` (no value) to mean "auto-pick a free port".
	c.Flags().Lookup("forward").NoOptDefVal = "0"
	return c
}

// runVNCForward opens the local-port tunnel and blocks until Ctrl-C. It prints
// the ready-to-use local vnc:// URL on stdout (one line, same as the non-forward
// path) and a human status on stderr.
func runVNCForward(cmd *cobra.Command, svc *internalrde.Service, workspaceID, sessionID string, localPort int, format output.Format) error {
	if format == output.JSON {
		return fmt.Errorf("--forward cannot be combined with --output json (it runs a long-lived tunnel, not a single-object result)")
	}
	// Fetch credentials up front so the local URL can carry them, and so a
	// session with no VNC endpoint fails clearly before we dial.
	creds, err := svc.GetSessionVNC(cmd.Context(), workspaceID, sessionID)
	if err != nil {
		return err
	}

	// Ctrl-C cancels the tunnel and returns cleanly rather than hard-killing.
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()

	onReady := func(localAddr string) {
		line := localAddr
		if host, portStr, splitErr := net.SplitHostPort(localAddr); splitErr == nil {
			if p, atoiErr := strconv.Atoi(portStr); atoiErr == nil {
				line = internalrde.FormatVNCURL(host, p, creds.Username, creds.Password)
			}
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
		if !cmdutil.IsQuiet(cmd) {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
				"Forwarding the session's VNC endpoint to %s — connect your VNC client there. Press Ctrl-C to stop.\n", localAddr)
		}
	}

	if err := svc.ForwardVNC(ctx, workspaceID, sessionID, localPort, onReady); err != nil {
		return err
	}
	if !cmdutil.IsQuiet(cmd) {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Stopped forwarding.")
	}
	return nil
}

func renderVNCCredentials(w io.Writer, creds internalrde.VNCCredentials) error {
	_, err := fmt.Fprintln(w, creds.URL)
	return err
}
