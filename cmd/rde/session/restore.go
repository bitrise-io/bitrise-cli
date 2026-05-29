package session

import (
	"fmt"

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
			svc := internalrde.NewService(client)
			sessionID, err := svc.ResolveSessionID(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}

			// A terminated session is restored from its persistent disk, so
			// check the disk first: fail fast with a clear reason when it's
			// gone, and warn when it's about to expire (the server still
			// allows the restore, so this is the only chance to surface it).
			sess, err := svc.GetSession(cmd.Context(), workspaceID, sessionID)
			if err != nil {
				return err
			}
			switch sess.PersistentDiskStatus {
			case internalrde.DiskStatusUnavailable:
				return fmt.Errorf("session %s cannot be restored: its persistent disk is no longer available", sessionID)
			case internalrde.DiskStatusUnavailableSoon:
				if !cmdutil.IsQuiet(cmd) && format != output.JSON {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
						"Warning: this session's persistent disk will become unavailable soon (within ~1 week) — restore while you still can.\n")
				}
			}

			restored, err := svc.RestoreSession(cmd.Context(), workspaceID, sessionID)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, restored, renderSessionDetail)
		},
	}
}
