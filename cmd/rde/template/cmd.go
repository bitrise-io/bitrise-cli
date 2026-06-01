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
		Long: `List and inspect RDE templates.

Commands that take a TEMPLATE_ID also accept a template name — it's resolved
to an ID for you. Names aren't unique, so if more than one template shares the
name the command errors and lists the candidate IDs to pick from.`,
		RunE: cmdutil.DelegateToList,
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
