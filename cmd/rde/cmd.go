// Package rde wires the `bitrise-cli rde ...` subcommand tree for the
// Bitrise Remote Dev Environments API.
package rde

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	rdeclaude "github.com/bitrise-io/bitrise-cli/cmd/rde/claude"
	rdedeviceguide "github.com/bitrise-io/bitrise-cli/cmd/rde/deviceguide"
	rdemachinetype "github.com/bitrise-io/bitrise-cli/cmd/rde/machinetype"
	rdepreviewlink "github.com/bitrise-io/bitrise-cli/cmd/rde/previewlink"
	rdesavedinput "github.com/bitrise-io/bitrise-cli/cmd/rde/savedinput"
	rdesession "github.com/bitrise-io/bitrise-cli/cmd/rde/session"
	rdestack "github.com/bitrise-io/bitrise-cli/cmd/rde/stack"
	rdetemplate "github.com/bitrise-io/bitrise-cli/cmd/rde/template"
	rdeusage "github.com/bitrise-io/bitrise-cli/cmd/rde/usage"
	rdewarmpool "github.com/bitrise-io/bitrise-cli/cmd/rde/warmpool"
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

Saved inputs are user-scoped — they do not require --workspace.

Device sessions: a session can boot an iOS simulator or Android emulator
('rde session create --device-platform ios|android'). Before creating one,
read 'rde device-guide' (then 'rde device-guide ios' or 'android'): it is the
know-how for waiting until the device is ready, connecting, driving it, and
what never to do.

Preview links: to let someone WITHOUT a Bitrise login try an app build in
their browser, mint a link with 'rde preview-link create' instead — typically
from CI, authenticated with a Workspace API Token.

Warm pools: 'rde warm-pool' keeps a number of sessions of one configuration
booted and idle so 'rde session create --warm-pool' (or a preview link minted
with --warm-pool) hands one out instantly instead of booting a VM.`,
		Example: `  bitrise-cli rde session list --workspace WORKSPACE_ID
  bitrise-cli rde session list --output json
  bitrise-cli rde template list
  bitrise-cli rde device-guide          # read before creating a session with a device
  bitrise-cli rde preview-link create --device-platform ios --artifact-url https://…/App.zip
  bitrise-cli rde warm-pool list`,
	}
	c.PersistentFlags().String(cmdutil.FlagWorkspace, "", "workspace ID (or set BITRISE_WORKSPACE_ID or default_workspace_id; auto-detected if you have exactly one workspace)")

	c.AddCommand(
		rdeclaude.NewCmd(),
		rdesession.NewCmd(),
		rdetemplate.NewCmd(),
		rdesavedinput.NewCmd(),
		rdestack.NewCmd(),
		rdemachinetype.NewCmd(),
		rdeusage.NewCmd(),
		rdepreviewlink.NewCmd(),
		rdewarmpool.NewCmd(),
		rdedeviceguide.NewCmd(),
	)
	return c
}
