// Package yml provides commands for managing the bitrise.yml stored on Bitrise.
package yml

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
)

// NewCmd returns the `bitrise-cli yml` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "yml",
		Short: "Get, update, or validate the bitrise.yml stored on Bitrise",
		Long: `Manage the bitrise.yml configuration stored on Bitrise.

Running 'bitrise-cli yml' without a subcommand defaults to 'yml get'.

Subcommands operate on the YAML stored server-side. If your project stores
bitrise.yml in the repository (version-controlled mode), get and update
commands still work, but uploaded changes will not affect builds.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, sub := range cmd.Commands() {
				if sub.Name() == "get" {
					sub.SetContext(cmd.Context())
					return sub.RunE(sub, args)
				}
			}
			return cmd.Help()
		},
	}
	c.PersistentFlags().String(cmdutil.FlagApp, "", "app ID (also accepted as --project; or set BITRISE_APP_ID)")
	c.SetGlobalNormalizationFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "project" {
			return pflag.NormalizedName(cmdutil.FlagApp)
		}
		return pflag.NormalizedName(name)
	})
	c.AddCommand(
		newGetCmd(),
		newUpdateCmd(),
		newValidateCmd(),
	)
	return c
}
