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
	var (
		assumeYes bool
		scope     string
	)
	c := &cobra.Command{
		Use:   "delete-terminated",
		Short: "Permanently delete your terminated sessions, or the workspace's with --scope workspace",
		Long: `Permanently delete your terminated sessions in this workspace, or the
workspace-owned ones with --scope workspace.

By default only sessions YOU created are affected: the server scopes the
call to the caller's own sessions, so other members' sessions and
workspace-owned sessions are never touched. Pass --scope workspace to
delete the terminated sessions owned by the workspace itself instead (the
ones 'rde session list --scope workspace' shows) — any member may do that.
With a Workspace API Token the workspace scope is the default and the only
one available. This cannot be undone. Pass --yes to skip the confirmation
prompt.`,
		Example: `  bitrise-cli rde session delete-terminated
  bitrise-cli rde session delete-terminated --scope workspace --yes`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch scope {
			case internalrde.SessionScopeMine, internalrde.SessionScopeWorkspace:
			default:
				return fmt.Errorf("--scope must be %s or %s", internalrde.SessionScopeMine, internalrde.SessionScopeWorkspace)
			}
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			if !assumeYes {
				prompt := "This will permanently delete YOUR terminated sessions in this workspace (other members' sessions are not affected).\nProceed? [y/N]: "
				if scope == internalrde.SessionScopeWorkspace {
					prompt = "This will permanently delete the terminated sessions OWNED BY THE WORKSPACE (visible to every member; personal sessions are not affected).\nProceed? [y/N]: "
				}
				if _, err := fmt.Fprint(cmd.ErrOrStderr(), prompt); err != nil {
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
			count, err := internalrde.NewService(client).DeleteTerminatedSessions(cmd.Context(), workspaceID, scope)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, deleteTerminatedResult{DeletedCount: count}, renderDeleteTerminated)
		},
	}
	c.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation prompt")
	c.Flags().StringVar(&scope, "scope", internalrde.SessionScopeMine, "which terminated sessions to delete: mine (sessions you created) or workspace (sessions owned by the workspace itself)")
	_ = c.RegisterFlagCompletionFunc("scope", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{internalrde.SessionScopeMine, internalrde.SessionScopeWorkspace}, cobra.ShellCompDirectiveNoFileComp
	})
	return c
}

func renderDeleteTerminated(w io.Writer, r deleteTerminatedResult) error {
	_, err := fmt.Fprintf(w, "Deleted %d terminated session(s)\n", r.DeletedCount)
	return err
}
