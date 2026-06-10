package build

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

func newAbortCmd() *cobra.Command {
	var (
		reason              string
		abortWithSuccess    bool
		skipGitStatusReport bool
		skipNotifications   bool
	)

	c := &cobra.Command{
		Use:   "abort BUILD_ID",
		Short: "Abort a running or queued build",
		Long: `Abort a running or queued build.

Required arguments:
  BUILD_ID           build ID to abort

Required flags:
  --app ID           (or BITRISE_APP_ID env var)

Optional flags:
  --reason STRING            reason for aborting (recorded in the build log)
  --abort-with-success       mark the aborted build as successful
  --skip-git-status-report   skip sending a git status report
  --skip-notifications       skip sending build notifications`,
		Args: cobra.ExactArgs(1),
		Example: `  bitrise-cli build abort BUILD_ID --app my-app-id
  bitrise-cli build abort BUILD_ID --app my-app-id --reason "no longer needed"
  bitrise-cli build abort BUILD_ID --app my-app-id --abort-with-success
  bitrise-cli build abort BUILD_ID --app my-app-id --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			buildSlug := args[0]
			appSlug, err := cmdutil.ResolveAppSlug(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}
			svc := internalbuild.NewService(client)
			result, err := svc.Abort(cmd.Context(), internalbuild.AbortRequest{
				AppSlug:             appSlug,
				BuildSlug:           buildSlug,
				Reason:              reason,
				AbortWithSuccess:    abortWithSuccess,
				SkipGitStatusReport: skipGitStatusReport,
				SkipNotifications:   skipNotifications,
			})
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, result, renderAbortHero)
		},
	}

	c.Flags().StringVar(&reason, "reason", "", "reason for aborting the build")
	c.Flags().BoolVar(&abortWithSuccess, "abort-with-success", false, "mark the aborted build as successful")
	c.Flags().BoolVar(&skipGitStatusReport, "skip-git-status-report", false, "skip sending a git status report")
	c.Flags().BoolVar(&skipNotifications, "skip-notifications", false, "skip sending build notifications")

	return c
}

func renderAbortHero(w io.Writer, r internalbuild.AbortResult) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)

	headline := strings.Builder{}
	headline.WriteString(s.Success.Render("✓"))
	headline.WriteString(" ")
	headline.WriteString(s.Bold.Render("Build aborted"))
	headline.WriteString(s.Dim.Render("  " + r.BuildSlug))
	ew.F("%s\n", headline.String())
	return ew.Err
}
