package session

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newRebuildCmd() *cobra.Command {
	var (
		wait        bool
		waitTimeout time.Duration
	)
	c := &cobra.Command{
		Use:   "rebuild SESSION_ID",
		Short: "Replace a session's machine and run its lifecycle hooks again from On create",
		Long: `Replace the machine of a session that runs lifecycle hooks and run its whole
setup again: the current VM AND its disk are discarded (anything unsaved on
it is lost), a fresh machine boots for the same session — same id, name,
labels and creation-time configuration (scripts, inputs, feature flags,
device) — and On create runs again from scratch, followed by the platform's
System setup and On claim ('session view' tags those runs "rebuild").

Use it when On create failed — it is the only way to run On create again
('session run-hook' does not accept it: On create runs once per machine) —
or when you want a clean environment with the same configuration. When a
failed On claim, On restore or System setup needs another go, 'session
run-hook' keeps the machine and everything on it.

Rebuild is asynchronous: by default the command returns while the session is
"starting". Pass --wait to block until it finishes provisioning (the command
exits non-zero if it ends in any state other than "running"); the hooks then
still run — watch "Setup:" in 'session view' until it says ready.

Refused on a warm pool's unclaimed sessions, on device preview sessions and
on sessions without lifecycle hooks.`,
		Example: `  bitrise-cli rde session rebuild SESSION_ID
  bitrise-cli rde session rebuild SESSION_ID --wait`,
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
			rebuilt, err := svc.RebuildSession(cmd.Context(), workspaceID, sessionID)
			if err != nil {
				return err
			}
			if wait {
				waitCtx, cancel := context.WithTimeout(cmd.Context(), waitTimeout)
				defer cancel()
				if !cmdutil.IsQuiet(cmd) && format != output.JSON {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Waiting for session %s to become ready (timeout %s)…\n", rebuilt.ID, waitTimeout)
				}
				ready, waitErr := svc.WaitForReady(waitCtx, workspaceID, rebuilt.ID, 0, nil)
				if waitErr != nil {
					return fmt.Errorf("waiting for session: %w", waitErr)
				}
				rebuilt = ready
				lifecycleErr := internalrde.LifecycleFailure(ready)
				if ready.Status != "running" || lifecycleErr != nil {
					if renderErr := output.Render(cmd.OutOrStdout(), format, rebuilt, renderSessionDetail); renderErr != nil {
						return renderErr
					}
					// No SilenceRootErrors: the rendered session doesn't say what
					// went wrong or which log to read, so the error goes to stderr
					// too — the only diagnostic with --output json.
					if lifecycleErr != nil {
						return lifecycleErr
					}
					return fmt.Errorf("session ended provisioning with status %q (expected running)", ready.Status)
				}
			}
			return output.Render(cmd.OutOrStdout(), format, rebuilt, renderSessionDetail)
		},
	}
	c.Flags().BoolVar(&wait, "wait", false, "wait until the new machine leaves provisioning (running, failed, …) and its hooks are done before returning; exits 1 if the final status isn't running or a hook failed")
	c.Flags().DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "max time to wait when --wait is set (Go duration syntax: 30s, 5m, 1h)")
	return c
}
