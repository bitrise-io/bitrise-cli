package session

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newTerminateCmd() *cobra.Command {
	var (
		wait        bool
		waitTimeout time.Duration
	)
	c := &cobra.Command{
		Use:   "terminate SESSION_ID",
		Short: "Terminate a running session but keep it for a later restore",
		Long: `Terminate a running session but keep it for a later restore.

The VM is stopped and its disk is preserved: the session stays in the list as
"terminated" and can be brought back with 'session restore'. Use this when you
intend to come back to this exact session (e.g. to keep uncommitted work or an
expensive warm state). A terminated session keeps using disk space until it is
deleted.

If you are simply done with the session, use 'session delete' instead — it
works on running sessions directly and frees the disk; no terminate needed.

Terminate is asynchronous: by default the command returns while the session
is still "terminating". Pass --wait to block until the session settles into a
terminal state ("terminated" or "failed"), e.g. before restoring it or when
you want the disk snapshot to be complete before moving on.

Terminating someone else's session: on a workspace where it is enabled (it is
enabled per workspace on request), whoever holds the workspace's billing-view
permission — the one 'rde usage' requires — can terminate any session in the
workspace: other members' personal sessions, workspace-owned sessions and
preview link sessions. 'rde usage --sessions' lists them with their IDs. Pass
the session ID: a session that is not yours cannot be found by name. It takes
a personal access token; a Workspace API Token does not get this wider right.

Only terminate is widened. The session is stopped, never deleted, and its
owner can restore it; viewing, deleting, restoring, exec, logs and the rest
still answer "not found" for a session that is not yours. For the same reason
--wait cannot follow such a session: the command warns and returns while it
is still "terminating". For someone else's session only its ID, name, status,
owner and timestamps come back, so that is all this command prints for it.

Where it is not enabled, the API refuses with 403 "Terminating another
member's session is not enabled for this workspace."; without the permission,
or for an unknown ID, it answers 404 as before.

A warm session a pool still holds (not claimed yet) cannot be terminated this
way; lower the pool's size instead ('rde warm-pool set-size WARM_POOL_ID 0').`,
		Example: `  bitrise-cli rde session terminate SESSION_ID
  bitrise-cli rde session terminate SESSION_ID --wait
  # Someone else's session: its ID is in the usage report
  bitrise-cli rde usage --sessions`,
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
			sess, err := svc.TerminateSession(cmd.Context(), workspaceID, sessionID)
			if err != nil {
				return err
			}

			if wait {
				waitCtx, cancel := context.WithTimeout(cmd.Context(), waitTimeout)
				defer cancel()
				if !cmdutil.IsQuiet(cmd) && format != output.JSON {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Waiting for session %s to terminate (timeout %s)…\n", sess.ID, waitTimeout)
				}
				settled, waitErr := svc.WaitForTerminated(waitCtx, workspaceID, sess.ID, 0)
				switch {
				case internalrde.IsNotFound(waitErr):
					// The terminate was accepted, but the session cannot be
					// read: it is someone else's (terminate is the only verb
					// that reaches it) or it was deleted meanwhile. Not a
					// failure — report what the terminate call returned.
					ew := cmdutil.NewErrWriter(cmd.ErrOrStderr())
					ew.F("%s cannot follow session %s (it is not yours to view, or it was deleted meanwhile); the terminate was accepted\n",
						style.New(cmd.ErrOrStderr()).Warn.Render("Warning:"), sess.ID)
					if ew.Err != nil {
						return ew.Err
					}
				case waitErr != nil:
					return fmt.Errorf("waiting for session to terminate: %w", waitErr)
				default:
					sess = settled
				}
			}

			return output.Render(cmd.OutOrStdout(), format, sess, renderSessionDetail)
		},
	}
	c.Flags().BoolVar(&wait, "wait", false, "block until the session settles into a terminal state (terminated/failed) before returning")
	c.Flags().DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "max time to wait when --wait is set (Go duration syntax: 30s, 5m, 1h)")
	return c
}
