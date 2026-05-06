package build

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
)

// NewCmd returns the `bitrise-cli build` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "build",
		Short: "Trigger, list, and inspect builds",
		RunE:  cmdutil.DelegateToList,
	}
	c.AddCommand(
		newTriggerCmd(),
		newListCmd(),
		newViewCmd(),
		newLogCmd(),
	)
	return c
}
