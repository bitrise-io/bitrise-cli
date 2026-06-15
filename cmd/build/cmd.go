package build

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
)

// NewCmd returns the `bitrise-cli build` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "build",
		Short: "Trigger, list, and inspect builds",
		Long: `Trigger, list, and inspect builds.

Builds belong to an app: pass --app ID on any build command, or set
BITRISE_APP_ID (or run "bitrise-cli config set app_id ID"). Running
"bitrise-cli build" with no subcommand lists builds for that app.`,
		Example: `  bitrise-cli build list --app APP_ID
  bitrise-cli build trigger --app APP_ID --branch main --workflow primary
  bitrise-cli build view --app APP_ID BUILD_ID --output json
  bitrise-cli build log --app APP_ID BUILD_ID`,
		RunE: cmdutil.DelegateToList,
	}
	c.PersistentFlags().String(cmdutil.FlagApp, "", "app ID (or set BITRISE_APP_ID)")
	c.AddCommand(
		newTriggerCmd(),
		newListCmd(),
		newViewCmd(),
		newLogCmd(),
		newWatchCmd(),
		newAbortCmd(),
		newYMLCmd(),
	)
	return c
}
