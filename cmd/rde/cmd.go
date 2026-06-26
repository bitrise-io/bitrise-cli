// Package rde wires the `bitrise-cli rde ...` subcommand tree for the
// Bitrise Remote Dev Environments API.
package rde

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	rdeclaude "github.com/bitrise-io/bitrise-cli/cmd/rde/claude"
	rdemachinetype "github.com/bitrise-io/bitrise-cli/cmd/rde/machinetype"
	rdesavedinput "github.com/bitrise-io/bitrise-cli/cmd/rde/savedinput"
	rdesession "github.com/bitrise-io/bitrise-cli/cmd/rde/session"
	rdestack "github.com/bitrise-io/bitrise-cli/cmd/rde/stack"
	rdetemplate "github.com/bitrise-io/bitrise-cli/cmd/rde/template"
)

// NewCmd returns the `bitrise-cli rde` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "rde",
		Short: "Manage Bitrise Remote Dev Environments (sessions, templates, …)",
		Long: `Manage Bitrise Remote Dev Environments — sessions, templates, saved inputs,
and the machine catalog (stacks, machine types).

Workspace resolution (highest to lowest precedence):
  --workspace ID            flag on the rde command
  BITRISE_WORKSPACE_ID      environment variable
  default_workspace_id      saved with 'bitrise-cli config set'
  auto-detect               when none of the above is set and you have exactly one workspace

Saved inputs are user-scoped — they do not require --workspace.`,
		Example: `  bitrise-cli rde session list --workspace WORKSPACE_ID
  bitrise-cli rde session list --output json
  bitrise-cli rde template list`,
	}
	c.PersistentFlags().String(cmdutil.FlagWorkspace, "", "workspace ID (or set BITRISE_WORKSPACE_ID or default_workspace_id; auto-detected if you have exactly one workspace)")

	c.AddCommand(
		rdeclaude.NewCmd(),
		rdesession.NewCmd(),
		rdetemplate.NewCmd(),
		rdesavedinput.NewCmd(),
		rdestack.NewCmd(),
		rdemachinetype.NewCmd(),
	)
	return c
}
