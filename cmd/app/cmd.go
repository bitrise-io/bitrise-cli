package app

import "github.com/spf13/cobra"

// NewCmd returns the `bitrise-cli app` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "app",
		Aliases: []string{"project"},
		Short:   "List, inspect, and manage apps (also: project)",
	}
	c.AddCommand(
		newListCmd(),
		newViewCmd(),
		newWorkflowCmd(),
	)
	return c
}
