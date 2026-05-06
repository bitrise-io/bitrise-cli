package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func newBuildViewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "view BUILD_SLUG",
		Short: "Show details of a single build",
		Long: `Show details for a single build identified by its build slug.

Required flags:
  --app SLUG         (or BITRISE_APP_SLUG env var)

Argument:
  BUILD_SLUG         the unique slug of the build (visible in build URLs)`,
		Example: `  bitrise-cli build view --app my-app-slug stub-build-aaa
  bitrise-cli build view --app my-app-slug stub-build-aaa --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appSlug, err := resolveAppSlug(cmd)
			if err != nil {
				return err
			}
			format := resolveFormat(cmd)

			svc := build.NewService()
			b, err := svc.View(cmd.Context(), appSlug, args[0])
			if err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), format, b, renderBuildText)
		},
	}

	c.Flags().String(flagApp, "", "app slug, alias: --project (or set BITRISE_APP_SLUG)")
	addAppProjectAlias(c)
	return c
}
