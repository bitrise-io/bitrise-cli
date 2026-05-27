package session

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore SESSION_ID",
		Short: "Restore a terminated session (re-provisions its VM from the persistent disk)",
		Args:  cmdutil.RequireArgs("SESSION_ID"),
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
			sess, err := internalrde.NewService(client).RestoreSession(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, sess, renderSessionDetail)
		},
	}
}
