package build

import "github.com/spf13/cobra"

// NewCmd returns the `bitrise-cli build` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "build",
		Short: "Trigger, list, and inspect builds",
	}
	c.AddCommand(
		newTriggerCmd(),
		newListCmd(),
		newViewCmd(),
		newLogCmd(),
	)
	return c
}
