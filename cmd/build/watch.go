package build

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func newWatchCmd() *cobra.Command {
	var interval time.Duration

	c := &cobra.Command{
		Use:   "watch BUILD_SLUG",
		Short: "Stream logs for a running build",
		Long: `Stream build logs until the build finishes, then exit with a status
reflecting the build outcome (0 = success, 1 = failed or aborted).

Ctrl-C detaches the CLI without affecting the running build.

Required flags:
  --app SLUG         (or BITRISE_APP_SLUG env var)

Argument:
  BUILD_SLUG         the unique slug of the build

Note:
  --output is ignored — logs are always streamed as raw text.`,
		Example: `  bitrise-cli build watch --app my-app-slug <build-slug>
  bitrise-cli build watch --app my-app-slug <build-slug> --interval 5s`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appSlug, err := cmdutil.ResolveAppSlug(cmd)
			if err != nil {
				return err
			}
			buildSlug := args[0]

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}
			svc := internalbuild.NewService(client)

			b, err := svc.View(cmd.Context(), appSlug, buildSlug)
			if err != nil {
				return err
			}
			return runWatch(cmd, svc, b, interval, cmd.OutOrStdout(), output.Human)
		},
	}

	c.Flags().String(cmdutil.FlagApp, "", "app slug, alias: --project (or set BITRISE_APP_SLUG)")
	c.Flags().DurationVar(&interval, "interval", 3*time.Second, "log polling interval")
	cmdutil.AddAppProjectAlias(c)
	return c
}
