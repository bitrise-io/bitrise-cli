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
		Use:   "log BUILD_SLUG",
		Short: "Print the build log",
		Long: `Print the log output for a single build.

Required flags:
  --app SLUG         (or BITRISE_APP_SLUG env var)

Argument:
  BUILD_SLUG         the unique slug of the build

Flags:
  --wait             wait for the build to finish before printing the log;
                     useful when the build is still in-progress. Ctrl-C
                     detaches without affecting the running build.
  --interval DURATION  polling interval when --wait is active (default 5s)

Note:
  --output is ignored — logs are streamed as raw text. Pipe to other tools or
  redirect to a file as needed.`,
		Example: `  bitrise-cli build log --app my-app-slug <build-slug>
  bitrise-cli build log --app my-app-slug <build-slug> --wait
  bitrise-cli build log --app my-app-slug <build-slug> --wait --interval 10s
  bitrise-cli build log --app my-app-slug <build-slug> > build.log`,
		Args: cmdutil.RequireArgs("BUILD_SLUG"),
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

			if wait {
				b, err := svc.View(cmd.Context(), appSlug, buildSlug)
				if err != nil {
					return err
				}
				if b.Status == "in-progress" {
					buildURL := b.BuildURL
					if buildURL == "" {
						buildURL = fmt.Sprintf("%s/app/%s/build/%s", cmdutil.WebBaseURL, appSlug, buildSlug)
					}
					headerEW := cmdutil.NewErrWriter(cmd.ErrOrStderr())
					headerEW.F("Waiting for build #%d to finish\n→ %s\n", b.BuildNumber, buildURL)
					if headerEW.Err != nil {
						return headerEW.Err
					}

					if _, err := svc.WaitForCompletion(cmd.Context(), appSlug, buildSlug, interval); err != nil {
						if errors.Is(err, context.Canceled) {
							detachEW := cmdutil.NewErrWriter(cmd.ErrOrStderr())
							detachEW.F("\nDetached — build is still running.\n")
							detachEW.F("Use 'bitrise-cli build log --wait %s' to resume.\n", buildSlug)
							return detachEW.Err
						}
						return err
					}
				}
			}

			return svc.Log(cmd.Context(), appSlug, buildSlug, cmd.OutOrStdout())
		},
	}

	c.Flags().BoolVar(&wait, "wait", false, "wait for the build to finish before printing the log")
	c.Flags().DurationVar(&interval, "interval", 5*time.Second, "polling interval when --wait is active")
	return c
}
