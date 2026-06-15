// Package step provides commands for searching steps and inspecting their inputs.
package step

import "github.com/spf13/cobra"

// NewCmd returns the `bitrise-cli step` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "step",
		Short: "Search steps and inspect their inputs",
		Long:  `Search the step library and inspect a step version's inputs.`,
		Example: `  bitrise-cli step search clone
  bitrise-cli step search fastlane --output json
  bitrise-cli step inputs git-clone@8.3.1`,
	}
	c.AddCommand(
		newSearchCmd(),
		newInputsCmd(),
	)
	return c
}
