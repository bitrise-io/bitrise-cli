package cmd

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func newBuildTriggerCmd() *cobra.Command {
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
			appSlug, err := resolveAppSlug(cmd)
			if err != nil {
				return err
			}
			format := resolveFormat(cmd)

			svc := build.NewService()
			b, err := svc.Trigger(cmd.Context(), build.TriggerRequest{
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

	c.Flags().String(flagApp, "", "app slug, alias: --project (or set BITRISE_APP_SLUG)")
	c.Flags().StringVar(&workflow, "workflow", "", "workflow ID to run (required)")
	c.Flags().StringVar(&branch, "branch", "main", "branch to build")
	c.Flags().StringVar(&commitHash, "commit-hash", "", "commit hash to build")
	c.Flags().StringVar(&commitMessage, "commit-message", "", "commit message to record")
	_ = c.MarkFlagRequired("workflow")
	addAppProjectAlias(c)

	return c
}

// renderBuildText prints a single build as key/value lines.
// Shared by trigger and view since both return one Build.
func renderBuildText(w io.Writer, b build.Build) error {
	ew := newErrWriter(w)
	ew.f("Build:        #%d (%s)\n", b.BuildNumber, b.Slug)
	ew.f("App:          %s\n", b.AppSlug)
	ew.f("Status:       %s\n", b.Status)
	if b.StatusText != "" {
		ew.f("Status Text:  %s\n", b.StatusText)
	}
	ew.f("Workflow:     %s\n", b.Workflow)
	ew.f("Branch:       %s\n", b.Branch)
	if b.CommitHash != "" {
		ew.f("Commit:       %s\n", b.CommitHash)
	}
	if b.CommitMessage != "" {
		ew.f("Message:      %s\n", b.CommitMessage)
	}
	ew.f("Triggered:    %s\n", b.TriggeredAt.Format("2006-01-02 15:04:05 MST"))
	if b.FinishedAt != nil {
		ew.f("Finished:     %s\n", b.FinishedAt.Format("2006-01-02 15:04:05 MST"))
	}
	if b.BuildURL != "" {
		ew.f("URL:          %s\n", b.BuildURL)
	}
	return ew.err
}
