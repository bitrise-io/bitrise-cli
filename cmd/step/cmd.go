// Package step provides commands for searching steps and inspecting their inputs.
package step

import "github.com/spf13/cobra"

// NewCmd returns the `bitrise-cli step` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "step",
		Short: "Search steps and inspect their inputs",
	}
	c.AddCommand(
		newSearchCmd(),
		newInputsCmd(),
	)
	return c
}
