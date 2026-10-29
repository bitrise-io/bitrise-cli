package warmpool

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newRetryCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "retry WARM_POOL_ID",
		Short: "Resume a warm pool paused after repeated On create failures",
		Long: `Resume a warm pool that paused after three consecutive On create failures
('warm-pool view' shows "Setup failed:" and 'list' says "setup failed"): the
machine kept from the last failure is deleted, the failure streak is cleared
and the pool starts warming again.

Do it after fixing what made On create fail — the template's script ('rde
template update') or something outside it (a token, a package mirror). Look at
the kept machine first: the session id on the "Setup failed:" line is a
session whose logs ('rde session logs --stage on-create') and terminal still
work. Any configuration change to the pool or its template resumes warming
the same way, so 'retry' is for "the script was fine, the world was not".

A pool that is not paused this way is returned unchanged.`,
		Example: `  bitrise-cli rde warm-pool retry ios-devs`,
		Args:    cmdutil.RequireArgs("WARM_POOL_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			svc := internalrde.NewService(client)
			poolID, err := svc.ResolveWarmPoolID(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			pool, err := svc.RetryWarmPool(cmd.Context(), workspaceID, poolID)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), cmdutil.ResolveFormat(cmd), pool, renderDetail)
		},
	}
	return c
}
