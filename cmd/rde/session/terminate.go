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

func newTerminateCmd() *cobra.Command {
	var (
		wait        bool
		waitTimeout time.Duration
	)
	c := &cobra.Command{
		Use:   "terminate SESSION_ID",
		Short: "Terminate a running session (preserves it for later restart)",
		Long: `Terminate a running session (preserves it for later restart).

Terminate is asynchronous: by default the command returns while the session
is still "terminating". Pass --wait to block until the session settles into a
terminal state ("terminated" or "failed"). This is what makes a
'terminate --wait && delete' pipeline reliable — delete rejects any session
that isn't yet terminated or failed.`,
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
			sess, err := svc.TerminateSession(cmd.Context(), workspaceID, args[0])
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
				if waitErr != nil {
					return fmt.Errorf("waiting for session to terminate: %w", waitErr)
				}
				sess = settled
			}

			return output.Render(cmd.OutOrStdout(), format, sess, renderSessionDetail)
		},
	}
	c.Flags().BoolVar(&wait, "wait", false, "block until the session settles into a terminal state (terminated/failed) before returning; makes 'terminate --wait && delete' reliable")
	c.Flags().DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "max time to wait when --wait is set (Go duration syntax: 30s, 5m, 1h)")
	return c
}
