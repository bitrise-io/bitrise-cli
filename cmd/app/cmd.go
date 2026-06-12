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
		Long: `List, inspect, and manage the apps you can access.

Running "bitrise-cli app" with no subcommand lists your apps.`,
		Example: `  bitrise-cli app
  bitrise-cli app list --output json
  bitrise-cli app view APP_ID
  bitrise-cli app create --repo-url https://github.com/acme/widget.git --workspace WORKSPACE_ID`,
		RunE: cmdutil.DelegateToList,
	}
	c.AddCommand(
		newListCmd(),
		newViewCmd(),
		newCreateCmd(),
	)
	return c
}
