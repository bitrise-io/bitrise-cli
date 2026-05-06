package build

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func newListCmd() *cobra.Command {
	var (
		branch           string
		workflow         string
		status           string
		sortBy           string
		commitMessage    string
		triggerEventType string
		pullRequestID    int
		buildNumber      int
		after            string
		before           string
		pipelineBuild    bool
		limit            int
		cursor           string
	)

	c := &cobra.Command{
		Use:   "list",
		Short: "List builds for an app",
		Long: `List builds for an app, newest first.

Required flags:
  --app SLUG              (or BITRISE_APP_SLUG env var)

Optional filters:
  --branch BRANCH
  --workflow ID
  --status STATUS           one of: success, failed, in-progress, aborted, aborted-with-success
  --sort-by ORDER           one of: created_at (default), running_first
  --commit-message TEXT     filter by commit message
  --trigger-event-type TYPE one of: push, pull-request, tag
  --pull-request-id N
  --build-number N
  --after RFC3339           builds triggered after this time (e.g. 2024-01-15T00:00:00Z)
  --before RFC3339          builds triggered before this time
  --pipeline-build          show only pipeline builds

Pagination:
  --limit N
  --cursor TOKEN          opaque token from a previous page's "next_cursor"`,
		Example: `  bitrise-cli build list --app my-app-slug
  bitrise-cli build list --app my-app-slug --branch main --status failed
  bitrise-cli build list --app my-app-slug --sort-by running_first
  bitrise-cli build list --app my-app-slug --after 2024-01-01T00:00:00Z
  bitrise-cli build list --app my-app-slug --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			appSlug, err := cmdutil.ResolveAppSlug(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}

			opts := internalbuild.ListOptions{
				AppSlug:          appSlug,
				Branch:           branch,
				Workflow:         workflow,
				Status:           status,
				SortBy:           sortBy,
				CommitMessage:    commitMessage,
				TriggerEventType: triggerEventType,
				PullRequestID:    pullRequestID,
				BuildNumber:      buildNumber,
				Limit:            limit,
				Cursor:           cursor,
			}
			if after != "" {
				t, err := time.Parse(time.RFC3339, after)
				if err != nil {
					return fmt.Errorf("--after: %w", err)
				}
				opts.After = &t
			}
			if before != "" {
				t, err := time.Parse(time.RFC3339, before)
				if err != nil {
					return fmt.Errorf("--before: %w", err)
				}
				opts.Before = &t
			}
			if cmd.Flags().Changed("pipeline-build") {
				opts.IsPipelineBuild = &pipelineBuild
			}

			svc := internalbuild.NewService(client)
			res, err := svc.List(cmd.Context(), opts)
			if err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), format, res, renderListText)
		},
	}

	c.Flags().String(cmdutil.FlagApp, "", "app slug, alias: --project (or set BITRISE_APP_SLUG)")
	c.Flags().StringVar(&branch, "branch", "", "filter by branch")
	c.Flags().StringVar(&workflow, "workflow", "", "filter by workflow ID")
	c.Flags().StringVar(&status, "status", "", "filter by status (success, failed, in-progress, aborted, aborted-with-success)")
	c.Flags().StringVar(&sortBy, "sort-by", "", "sort order: created_at (default) or running_first")
	c.Flags().StringVar(&commitMessage, "commit-message", "", "filter by commit message")
	c.Flags().StringVar(&triggerEventType, "trigger-event-type", "", "filter by trigger event type")
	c.Flags().IntVar(&pullRequestID, "pull-request-id", 0, "filter by pull request ID")
	c.Flags().IntVar(&buildNumber, "build-number", 0, "filter by build number")
	c.Flags().StringVar(&after, "after", "", "show builds triggered after this time (RFC3339, e.g. 2024-01-15T00:00:00Z)")
	c.Flags().StringVar(&before, "before", "", "show builds triggered before this time (RFC3339, e.g. 2024-01-15T00:00:00Z)")
	c.Flags().BoolVar(&pipelineBuild, "pipeline-build", false, "show only pipeline builds (omit to show all)")
	c.Flags().IntVar(&limit, "limit", 0, "max items per page (server default if 0)")
	c.Flags().StringVar(&cursor, "cursor", "", "pagination cursor from a previous response")
	cmdutil.AddAppProjectAlias(c)

	return c
}

func renderListText(w io.Writer, res internalbuild.ListResult) error {
	if len(res.Items) == 0 {
		_, err := fmt.Fprintln(w, "No builds found.")
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	ew := cmdutil.NewErrWriter(tw)
	ew.Ln("NUMBER\tSTATUS\tBRANCH\tWORKFLOW\tTRIGGERED\tSLUG")
	for _, b := range res.Items {
		status := b.Status
		if b.IsOnHold {
			status += " (held)"
		}
		ew.F("%d\t%s\t%s\t%s\t%s\t%s\n",
			b.BuildNumber, status, b.Branch, b.Workflow,
			b.TriggeredAt.Format("2006-01-02 15:04"), b.Slug,
		)
	}
	if ew.Err != nil {
		return ew.Err
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if res.NextCursor != "" {
		_, err := fmt.Fprintf(w, "\nMore results available — pass --cursor %s\n", res.NextCursor)
		return err
	}
	return nil
}
