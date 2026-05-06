package app

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
)

func newWorkflowCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "workflow",
		Short: "Inspect workflows defined on an app",
		RunE:  cmdutil.DelegateToList,
	}
	c.AddCommand(newWorkflowListCmd())
	return c
}
