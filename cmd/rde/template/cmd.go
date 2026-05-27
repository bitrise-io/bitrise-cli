// Package template wires `bitrise-cli rde template` subcommands.
package template

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
)

// NewCmd returns the `rde template` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "template",
		Short: "List and inspect RDE templates",
		RunE:  cmdutil.DelegateToList,
	}
	c.AddCommand(
		newListCmd(),
		newViewCmd(),
		newCreateCmd(),
		newUpdateCmd(),
		newDeleteCmd(),
	)
	return c
}
