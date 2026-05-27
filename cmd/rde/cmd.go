// Package rde wires the `bitrise-cli rde ...` subcommand tree for the
// Bitrise Remote Dev Environments API.
package rde

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	rdecluster "github.com/bitrise-io/bitrise-cli/cmd/rde/cluster"
	rdeimage "github.com/bitrise-io/bitrise-cli/cmd/rde/image"
	rdemachinetype "github.com/bitrise-io/bitrise-cli/cmd/rde/machinetype"
	rdesavedinput "github.com/bitrise-io/bitrise-cli/cmd/rde/savedinput"
	rdesession "github.com/bitrise-io/bitrise-cli/cmd/rde/session"
	rdetemplate "github.com/bitrise-io/bitrise-cli/cmd/rde/template"
)

// NewCmd returns the `bitrise-cli rde` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "rde",
		Short: "Manage Bitrise Remote Dev Environments (sessions, templates, …)",
		Long: `Manage Bitrise Remote Dev Environments — sessions, templates, saved inputs,
and the machine catalog (images, machine types, clusters).

Workspace resolution (highest to lowest precedence):
  --workspace SLUG          flag on the rde command
  BITRISE_WORKSPACE_ID      environment variable
  default_organization_slug saved with 'bitrise-cli config set'

Saved inputs are user-scoped — they do not require --workspace.`,
	}
	c.PersistentFlags().String(cmdutil.FlagWorkspace, "", "workspace slug (or set BITRISE_WORKSPACE_ID; defaults to default_organization_slug)")

	c.AddCommand(
		rdesession.NewCmd(),
		rdetemplate.NewCmd(),
		rdesavedinput.NewCmd(),
		rdeimage.NewCmd(),
		rdemachinetype.NewCmd(),
		rdecluster.NewCmd(),
	)
	return c
}
