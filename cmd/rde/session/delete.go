package session

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete SESSION_ID",
		Short: "Permanently delete a session",
		Args:  cmdutil.RequireArgs("SESSION_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			if err := internalrde.NewService(client).DeleteSession(cmd.Context(), workspaceID, args[0]); err != nil {
				return err
			}
			if !cmdutil.IsQuiet(cmd) {
				_, err := fmt.Fprintf(cmd.ErrOrStderr(), "Deleted session %s\n", args[0])
				return err
			}
			return nil
		},
	}
}
