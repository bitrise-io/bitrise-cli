// Package session wires `bitrise-cli rde session` subcommands.
package session

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
)

// NewCmd returns the `rde session` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "session",
		Short: "Create, list, inspect, and manage RDE sessions",
		RunE:  cmdutil.DelegateToList,
	}
	c.AddCommand(
		newListCmd(),
		newViewCmd(),
		newCreateCmd(),
		newUpdateCmd(),
		newRestoreCmd(),
		newTerminateCmd(),
		newDeleteCmd(),
		newDeleteTerminatedCmd(),
		newDiffCmd(),
		newNotificationsCmd(),
		newExecCmd(),
		newUploadCmd(),
		newDownloadCmd(),
	)
	return c
}
