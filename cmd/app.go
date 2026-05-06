package cmd

import "github.com/spf13/cobra"

// newAppCmd returns the `bitrise-cli app` parent command.
//
// "project" is registered as an alias because Bitrise's UI uses "project"
// while the API and slug system use "app"; both names work to ease the
// transition. The patterns guide flags this as an explicit team call.
func newAppCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "app",
		Aliases: []string{"project"},
		Short:   "List, inspect, and manage apps (also: project)",
	}
	c.AddCommand(
		newAppListCmd(),
		newAppViewCmd(),
		newAppWorkflowCmd(),
	)
	return c
}
