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
		Long: `Create, list, inspect, and manage RDE sessions.

Commands that take a SESSION_ID also accept a session name — it's resolved to
an ID for you. Names aren't unique, so if more than one session shares the name
the command errors and lists the candidate IDs to pick from.`,
		RunE: cmdutil.DelegateToList,
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
		newVNCCmd(),
		newOpenVNCCmd(),
	)
	return c
}
