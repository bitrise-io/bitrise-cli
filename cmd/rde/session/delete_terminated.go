package session

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

// deleteTerminatedResult is the --output json shape: {"deleted_count": N}.
type deleteTerminatedResult struct {
	DeletedCount int `json:"deleted_count"`
}

func newDeleteTerminatedCmd() *cobra.Command {
	var assumeYes bool
	c := &cobra.Command{
		Use:   "delete-terminated",
		Short: "Permanently delete every terminated session in the workspace",
		Long: `Permanently delete every terminated session in the workspace.
This cannot be undone. Pass --yes to skip the confirmation prompt.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			if !assumeYes {
				if _, err := fmt.Fprint(cmd.ErrOrStderr(),
					"This will permanently delete every terminated session in the workspace.\nProceed? [y/N]: "); err != nil {
					return err
				}
				answer, err := cmdutil.ReadSecretInput(cmd.InOrStdin(), cmd.ErrOrStderr(), "", true)
				if err != nil {
					return err
				}
				if answer != "y" && answer != "Y" && answer != "yes" {
					return fmt.Errorf("aborted")
				}
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			count, err := internalrde.NewService(client).DeleteTerminatedSessions(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, deleteTerminatedResult{DeletedCount: count}, renderDeleteTerminated)
		},
	}
	c.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation prompt")
	return c
}

func renderDeleteTerminated(w io.Writer, r deleteTerminatedResult) error {
	_, err := fmt.Fprintf(w, "Deleted %d terminated session(s)\n", r.DeletedCount)
	return err
}
