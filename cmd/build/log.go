package build

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
)

func newLogCmd() *cobra.Command {
	var (
		wait     bool
		interval time.Duration
	)

	c := &cobra.Command{
		Use:   "log BUILD_ID",
		Short: "Print the build log",
		Long: `Print the log output for a single build.

Required flags:
  --app ID           (or BITRISE_APP_ID env var)

Argument:
  BUILD_ID           the unique ID of the build

Flags:
  --wait             wait for the build to finish before printing the log;
                     useful when the build is still in-progress. Ctrl-C
                     detaches without affecting the running build. Exit status
                     reflects log retrieval, not the build outcome — use
                     'build watch' to gate on build success/failure.
  --interval DURATION  polling interval when --wait is active (default 3s)

Note:
  --output is ignored — logs are streamed as raw text. Pipe to other tools or
  redirect to a file as needed.`,
		Example: `  bitrise-cli build log --app my-app-id <build-id>
  bitrise-cli build log --app my-app-id <build-id> --wait
  bitrise-cli build log --app my-app-id <build-id> --wait --interval 10s
  bitrise-cli build log --app my-app-id <build-id> > build.log`,
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

			svc := internalbuild.NewService(client)

			if wait {
				b, err := svc.View(cmd.Context(), appSlug, buildSlug)
				if err != nil {
					return err
				}
				if b.Status == "in-progress" {
					if !cmdutil.IsQuiet(cmd) {
						buildURL := b.BuildURL
						if buildURL == "" {
							buildURL = fmt.Sprintf("%s/app/%s/build/%s", cmdutil.ResolveWebBaseURL(cmd), appSlug, buildSlug)
						}
						headerEW := cmdutil.NewErrWriter(cmd.ErrOrStderr())
						headerEW.F("Waiting for build #%d to finish\n→ %s\n", b.BuildNumber, buildURL)
						if headerEW.Err != nil {
							return headerEW.Err
						}
					}

					if _, err := svc.WaitForCompletion(cmd.Context(), appSlug, buildSlug, interval); err != nil {
						if errors.Is(err, context.Canceled) {
							return writeDetachNotice(cmd.ErrOrStderr(), "build log --wait "+buildSlug)
						}
						return err
					}
				}
			}

			return svc.Log(cmd.Context(), appSlug, buildSlug, cmd.OutOrStdout())
		},
	}

	c.Flags().BoolVar(&wait, "wait", false, "wait for the build to finish before printing the log")
	c.Flags().DurationVar(&interval, "interval", 3*time.Second, "polling interval when --wait is active")
	return c
}
