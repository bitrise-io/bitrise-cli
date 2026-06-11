package app

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
)

// NewCmd returns the `bitrise-cli app` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "app",
		Short: "List, inspect, and manage apps",
		RunE:  cmdutil.DelegateToList,
	}
	c.AddCommand(
		newListCmd(),
		newViewCmd(),
		newCreateCmd(),
	)
	return c
}
