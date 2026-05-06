package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/internal/build"
)

func newBuildLogCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "log BUILD_SLUG",
		Short: "Print the build log",
		Long: `Print the log output for a single build.

Required flags:
  --app SLUG         (or BITRISE_APP_SLUG env var)

Argument:
  BUILD_SLUG         the unique slug of the build

Note:
  --output is ignored — logs are streamed as raw text. Pipe to other tools or
  redirect to a file as needed.`,
		Example: `  bitrise-cli build log --app my-app-slug stub-build-aaa
  bitrise-cli build log --app my-app-slug stub-build-aaa > build.log`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appSlug, err := resolveAppSlug(cmd)
			if err != nil {
				return err
			}

			svc := build.NewService()
			return svc.Log(cmd.Context(), appSlug, args[0], cmd.OutOrStdout())
		},
	}

	c.Flags().String(flagApp, "", "app slug, alias: --project (or set BITRISE_APP_SLUG)")
	addAppProjectAlias(c)
	return c
}
