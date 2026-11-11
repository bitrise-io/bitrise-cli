// Package workspacescript wires `bitrise-cli rde workspace-script`
// subcommands.
package workspacescript

import (
	"github.com/spf13/cobra"
)

// NewCmd returns the `rde workspace-script` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "workspace-script",
		Short: "View and edit the script every session in the workspace boots with",
		Long: `View and edit the workspace script: one bash script per workspace that
every session boot runs first — every session (from a template, ad hoc, from
a warm pool, device sessions), every boot (including restore and rebuild),
on macOS and Linux. It runs as the session user from $HOME via 'bash -l',
with its variables in the environment, and its output goes to the session's
System log, each line prefixed "[workspace] ".

The template's lifecycle hooks or startup script wait until the workspace
script exits, at most 10 minutes; after that they start anyway and the
script keeps running (it is never killed). A line '# rde: no-wait-limit' in
the script lifts the limit. Start long-running processes in the background
('my-agent &') so the script can exit.

Disabled is draft mode, not off: the script stays saved and runs only on
test sessions — personal sessions created with 'bitrise-cli rde session
create NAME --workspace-script-test'. Test the script that way before
enabling it: once enabled, every member's sessions run it at their next
boot.

Variables are plain (value shown), secret (encrypted at rest, shown only
with --show-secrets) or protected (encrypted and write-only: never shown,
only whether a value is set). Keys starting with RDE_ are reserved.

Only people with the workspace's Usage permission (workspace owners and
billing managers) can view or edit the script, with a personal access token.
Workspace API Tokens are refused by default; if your automation needs one
for this, contact Bitrise. The feature must be enabled for the workspace by
Bitrise.`,
		Example: `  bitrise-cli rde workspace-script view
  bitrise-cli rde workspace-script update --script-file workspace-script.sh
  bitrise-cli rde session create try-it --workspace-script-test --template TEMPLATE_ID
  bitrise-cli rde workspace-script update --enable`,
	}
	c.AddCommand(
		newViewCmd(),
		newUpdateCmd(),
	)
	return c
}
