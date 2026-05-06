package cmd

import "github.com/spf13/cobra"

// newAppWorkflowCmd returns the `bitrise-cli app workflow` parent command.
func newAppWorkflowCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "workflow",
		Short: "Inspect workflows defined on an app",
	}
	c.AddCommand(newAppWorkflowListCmd())
	return c
}
