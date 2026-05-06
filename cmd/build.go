package cmd

import "github.com/spf13/cobra"

// newBuildCmd returns the `bitrise-cli build` parent command.
// Subcommands handle the actual work; the parent only groups them.
func newBuildCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "build",
		Short: "Trigger, list, and inspect builds",
	}
	c.AddCommand(
		newBuildTriggerCmd(),
		newBuildListCmd(),
		newBuildViewCmd(),
		newBuildLogCmd(),
	)
	return c
}
