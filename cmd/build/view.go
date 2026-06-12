package build

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func newViewCmd() *cobra.Command {
	var web bool

	c := &cobra.Command{
		Use:   "view BUILD_ID",
		Short: "Show details of a single build",
		Long: `Show details for a single build identified by its build ID.

Required flags:
  --app ID           (or BITRISE_APP_ID env var)

Argument:
  BUILD_ID           the unique ID of the build (visible in build URLs)

Flags:
  --web              open the build page in the browser instead of printing`,
		Example: `  bitrise-cli build view --app my-app-id stub-build-aaa
  bitrise-cli build view --app my-app-id stub-build-aaa --output json
  bitrise-cli build view --app my-app-id stub-build-aaa --web`,
		Args: cmdutil.RequireArgs("BUILD_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}
			appSlug, err := cmdutil.ResolveAndLookupAppSlug(cmd, client)
			if err != nil {
				return err
			}
			buildSlug := args[0]

			if web {
				url := fmt.Sprintf("%s/app/%s/build/%s", cmdutil.ResolveWebBaseURL(cmd), appSlug, buildSlug)
				if err := cmdutil.OpenBrowser(url); err != nil {
					return err
				}
				if !cmdutil.IsQuiet(cmd) {
					_, err := fmt.Fprintf(cmd.ErrOrStderr(), "Opened %s\n", url)
					if err != nil {
						return err
					}
				}
				return nil
			}

			format := cmdutil.ResolveFormat(cmd)

			svc := internalbuild.NewService(client)
			b, err := svc.View(cmd.Context(), appSlug, buildSlug)
			if err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), format, b, renderBuildText)
		},
	}

	c.Flags().BoolVar(&web, cmdutil.FlagWeb, false, "open the build page in the browser")
	return c
}
