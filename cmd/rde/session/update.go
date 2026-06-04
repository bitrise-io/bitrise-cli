package session

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newUpdateCmd() *cobra.Command {
	var (
		name                 string
		description          string
		autoTerminateMinutes int
	)
	c := &cobra.Command{
		Use:   "update SESSION_ID",
		Short: "Update a session's name, description, or auto-terminate duration",
		Args:  cmdutil.RequireArgs("SESSION_ID"),
		Example: `  bitrise-cli rde session update SESSION_ID --name new-name
  bitrise-cli rde session update SESSION_ID --auto-terminate-minutes 0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			req := internalrde.UpdateSessionRequest{}
			if cmd.Flags().Changed("name") {
				n := name
				req.Name = &n
			}
			if cmd.Flags().Changed("description") {
				d := description
				req.Description = &d
			}
			if cmd.Flags().Changed("auto-terminate-minutes") {
				m := autoTerminateMinutes
				req.AutoTerminateMinutes = &m
			}
			if req.Name == nil && req.Description == nil && req.AutoTerminateMinutes == nil {
				return fmt.Errorf("at least one of --name, --description, --auto-terminate-minutes is required")
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
			sess, err := svc.UpdateSession(cmd.Context(), workspaceID, sessionID, req)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, sess, renderSessionDetail)
		},
	}
	c.Flags().StringVar(&name, "name", "", "new session name")
	c.Flags().StringVar(&description, "description", "", "new session description")
	c.Flags().IntVar(&autoTerminateMinutes, "auto-terminate-minutes", 0, "auto-terminate duration in minutes; 0 disables. Resets the deadline to now + minutes.")
	return c
}
