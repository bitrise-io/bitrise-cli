package build

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func newTriggerCmd() *cobra.Command {
	var (
		workflow      string
		branch        string
		commitHash    string
		commitMessage string
	)

	c := &cobra.Command{
		Use:   "trigger",
		Short: "Start a new build",
		Long: `Start a new build on the given app.

Required flags:
  --app SLUG         (or BITRISE_APP_SLUG env var)
  --workflow ID

Optional flags:
  --branch BRANCH    defaults to "main"
  --commit-hash HASH
  --commit-message MSG`,
		Example: `  bitrise-cli build trigger --app my-app-slug --workflow primary
  bitrise-cli build trigger --app my-app-slug --workflow deploy --branch release/1.2 --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			appSlug, err := cmdutil.ResolveAppSlug(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)

			svc := internalbuild.NewService()
			b, err := svc.Trigger(cmd.Context(), internalbuild.TriggerRequest{
				AppSlug:       appSlug,
				Workflow:      workflow,
				Branch:        branch,
				CommitHash:    commitHash,
				CommitMessage: commitMessage,
			})
			if err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), format, b, renderBuildText)
		},
	}

	c.Flags().String(cmdutil.FlagApp, "", "app slug, alias: --project (or set BITRISE_APP_SLUG)")
	c.Flags().StringVar(&workflow, "workflow", "", "workflow ID to run (required)")
	c.Flags().StringVar(&branch, "branch", "main", "branch to build")
	c.Flags().StringVar(&commitHash, "commit-hash", "", "commit hash to build")
	c.Flags().StringVar(&commitMessage, "commit-message", "", "commit message to record")
	_ = c.MarkFlagRequired("workflow")
	cmdutil.AddAppProjectAlias(c)

	return c
}
