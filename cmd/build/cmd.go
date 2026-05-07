package build

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
)

// NewCmd returns the `bitrise-cli build` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "build",
		Short: "Trigger, list, and inspect builds",
		RunE:  cmdutil.DelegateToList,
	}
	c.PersistentFlags().String(cmdutil.FlagApp, "", "app slug (also accepted as --project; or set BITRISE_APP_SLUG)")
	c.SetGlobalNormalizationFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "project" {
			return pflag.NormalizedName(cmdutil.FlagApp)
		}
		return pflag.NormalizedName(name)
	})
	c.AddCommand(
		newTriggerCmd(),
		newListCmd(),
		newViewCmd(),
		newLogCmd(),
		newWatchCmd(),
		newAbortCmd(),
	)
	return c
}
