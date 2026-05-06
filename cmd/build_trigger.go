package cmd

import (
	"fmt"
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
	fmt.Fprintf(w, "Build:        #%d (%s)\n", b.BuildNumber, b.Slug)
	fmt.Fprintf(w, "App:          %s\n", b.AppSlug)
	fmt.Fprintf(w, "Status:       %s\n", b.Status)
	if b.StatusText != "" {
		fmt.Fprintf(w, "Status Text:  %s\n", b.StatusText)
	}
	fmt.Fprintf(w, "Workflow:     %s\n", b.Workflow)
	fmt.Fprintf(w, "Branch:       %s\n", b.Branch)
	if b.CommitHash != "" {
		fmt.Fprintf(w, "Commit:       %s\n", b.CommitHash)
	}
	if b.CommitMessage != "" {
		fmt.Fprintf(w, "Message:      %s\n", b.CommitMessage)
	}
	fmt.Fprintf(w, "Triggered:    %s\n", b.TriggeredAt.Format("2006-01-02 15:04:05 MST"))
	if b.FinishedAt != nil {
		fmt.Fprintf(w, "Finished:     %s\n", b.FinishedAt.Format("2006-01-02 15:04:05 MST"))
	}
	if b.BuildURL != "" {
		fmt.Fprintf(w, "URL:          %s\n", b.BuildURL)
	}
	return nil
}
