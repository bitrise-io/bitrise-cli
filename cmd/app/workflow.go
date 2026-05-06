package app

import "github.com/spf13/cobra"

func newWorkflowCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "workflow",
		Short: "Inspect workflows defined on an app",
	}
	c.AddCommand(newWorkflowListCmd())
	return c
}
