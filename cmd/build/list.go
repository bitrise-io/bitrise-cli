package build

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
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
		fetchAll         bool
	)

	c := &cobra.Command{
		Use:   "list",
		Short: "List builds for an app",
		Long: `List builds for an app, newest first.

Required flags:
  --app SLUG              (or BITRISE_APP_SLUG env var)

Optional filters:
  --branch BRANCH           filter by branch name
  --workflow ID             filter by workflow ID
  --status STATUS           one of: success, failed, in-progress, aborted, aborted-with-success
  --sort-by ORDER           one of: created_at (default), running_first
  --commit-message TEXT     filter by commit message
  --trigger-event-type TYPE one of: push, pull-request, tag
  --pull-request-id N       filter by pull request ID
  --build-number N          filter by build number
  --after RFC3339           builds triggered after this time (e.g. 2024-01-15T00:00:00Z)
  --before RFC3339          builds triggered before this time
  --pipeline-build          show only pipeline builds

Pagination:
  --limit N               max items per page (server default if 0)
  --cursor TOKEN          opaque token from a previous page's next_cursor
  --all                   fetch all pages automatically

In JSON mode (--output json), the next_cursor field holds the cursor value for scripting:
  bitrise-cli build list --app SLUG --output json | jq -r '.next_cursor'`,
		Example: `  bitrise-cli build list --app my-app-slug
  bitrise-cli build list --app my-app-slug --all
  bitrise-cli build list --app my-app-slug --branch main --status failed
  bitrise-cli build list --app my-app-slug --sort-by running_first
  bitrise-cli build list --app my-app-slug --after 2024-01-01T00:00:00Z
  bitrise-cli build list --app my-app-slug --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if fetchAll && cursor != "" {
				return fmt.Errorf("--all and --cursor cannot be used together")
			}

			appSlug, err := cmdutil.ResolveAppSlug(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}

			var afterTime, beforeTime *time.Time
			if after != "" {
				t, err := time.Parse(time.RFC3339, after)
				if err != nil {
					return fmt.Errorf("--after: %w", err)
				}
				afterTime = &t
			}
			if before != "" {
				t, err := time.Parse(time.RFC3339, before)
				if err != nil {
					return fmt.Errorf("--before: %w", err)
				}
				beforeTime = &t
			}
			var isPipelineBuild *bool
			if cmd.Flags().Changed("pipeline-build") {
				isPipelineBuild = &pipelineBuild
			}

			makeOpts := func(cur string) internalbuild.ListOptions {
				return internalbuild.ListOptions{
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
					Cursor:           cur,
					After:            afterTime,
					Before:           beforeTime,
					IsPipelineBuild:  isPipelineBuild,
				}
			}

			svc := internalbuild.NewService(client)

			var res internalbuild.ListResult
			if fetchAll {
				var allItems []internalbuild.Build
				cur := ""
				for {
					page, pageErr := svc.List(cmd.Context(), makeOpts(cur))
					if pageErr != nil {
						return pageErr
					}
					allItems = append(allItems, page.Items...)
					if page.NextCursor == "" {
						break
					}
					cur = page.NextCursor
				}
				res = internalbuild.ListResult{Items: allItems}
			} else {
				res, err = svc.List(cmd.Context(), makeOpts(cursor))
				if err != nil {
					return err
				}
			}

			nextPageCmd := func(nextCursor string) string {
				parts := []string{"bitrise-cli build list", "--app", appSlug}
				if cmd.Flags().Changed("branch") {
					parts = append(parts, "--branch", branch)
				}
				if cmd.Flags().Changed("workflow") {
					parts = append(parts, "--workflow", workflow)
				}
				if cmd.Flags().Changed("status") {
					parts = append(parts, "--status", status)
				}
				if cmd.Flags().Changed("sort-by") {
					parts = append(parts, "--sort-by", sortBy)
				}
				if cmd.Flags().Changed("commit-message") {
					parts = append(parts, "--commit-message", commitMessage)
				}
				if cmd.Flags().Changed("trigger-event-type") {
					parts = append(parts, "--trigger-event-type", triggerEventType)
				}
				if cmd.Flags().Changed("pull-request-id") {
					parts = append(parts, "--pull-request-id", strconv.Itoa(pullRequestID))
				}
				if cmd.Flags().Changed("build-number") {
					parts = append(parts, "--build-number", strconv.Itoa(buildNumber))
				}
				if cmd.Flags().Changed("after") {
					parts = append(parts, "--after", after)
				}
				if cmd.Flags().Changed("before") {
					parts = append(parts, "--before", before)
				}
				if cmd.Flags().Changed("pipeline-build") {
					parts = append(parts, "--pipeline-build="+strconv.FormatBool(pipelineBuild))
				}
				if cmd.Flags().Changed("limit") {
					parts = append(parts, "--limit", strconv.Itoa(limit))
				}
				parts = append(parts, "--cursor", nextCursor)
				return strings.Join(parts, " ")
			}

			render := func(w io.Writer, r internalbuild.ListResult) error {
				return renderListText(w, r, nextPageCmd)
			}
			return output.Render(cmd.OutOrStdout(), format, res, render)
		},
	}

	c.Flags().StringVar(&branch, "branch", "", "filter by branch")
	c.Flags().StringVar(&workflow, "workflow", "", "filter by workflow ID")
	c.Flags().StringVar(&status, "status", "", "filter by status (success, failed, in-progress, aborted, aborted-with-success)")
	c.Flags().StringVar(&sortBy, "sort-by", "", "sort order: created_at (default) or running_first")
	c.Flags().StringVar(&commitMessage, "commit-message", "", "filter by commit message")
	c.Flags().StringVar(&triggerEventType, "trigger-event-type", "", "filter by trigger event type (push, pull-request, tag)")
	c.Flags().IntVar(&pullRequestID, "pull-request-id", 0, "filter by pull request ID")
	c.Flags().IntVar(&buildNumber, "build-number", 0, "filter by build number")
	c.Flags().StringVar(&after, "after", "", "show builds triggered after this time (RFC3339, e.g. 2024-01-15T00:00:00Z)")
	c.Flags().StringVar(&before, "before", "", "show builds triggered before this time (RFC3339, e.g. 2024-01-15T00:00:00Z)")
	c.Flags().BoolVar(&pipelineBuild, "pipeline-build", false, "show only pipeline builds (omit to show all)")
	c.Flags().IntVar(&limit, "limit", 0, "max items per page (server default if 0)")
	c.Flags().StringVar(&cursor, "cursor", "", "pagination cursor from a previous response")
	c.Flags().BoolVar(&fetchAll, "all", false, "fetch all pages automatically")

	_ = c.RegisterFlagCompletionFunc("status", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"success\tbuild completed successfully",
			"failed\tbuild failed",
			"in-progress\tbuild is currently running",
			"aborted\tbuild was aborted",
			"aborted-with-success\tbuild was aborted but considered successful",
		}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = c.RegisterFlagCompletionFunc("sort-by", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"created_at\tnewest builds first (default)",
			"running_first\trunning builds appear before finished ones",
		}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = c.RegisterFlagCompletionFunc("trigger-event-type", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"push", "pull-request", "tag"}, cobra.ShellCompDirectiveNoFileComp
	})

	return c
}

func renderListText(w io.Writer, res internalbuild.ListResult, nextPageCmd func(string) string) error {
	if len(res.Items) == 0 {
		_, err := fmt.Fprintln(w, "No builds found.")
		return err
	}

	s := style.New(w)
	headers := []string{"NUMBER", "STATUS", "BRANCH", "WORKFLOW", "TRIGGERED", "SLUG"}
	rows := make([][]string, 0, len(res.Items))
	statuses := make([]string, 0, len(res.Items))
	for _, b := range res.Items {
		status := b.Status
		if b.IsOnHold {
			status += " (held)"
		}
		statuses = append(statuses, b.Status)
		rows = append(rows, []string{
			strconv.Itoa(b.BuildNumber),
			status,
			b.Branch,
			b.Workflow,
			b.TriggeredAt.Format("2006-01-02 15:04"),
			b.Slug,
		})
	}
	const (
		colStatus = 1
		colSlug   = 5
	)
	styler := func(row, col int, content string) string {
		switch col {
		case colStatus:
			return s.BuildStatus(statuses[row]).Render(content)
		case colSlug:
			return s.Slug.Render(content)
		}
		return content
	}
	if err := style.Table(w, headers, rows, s.Header, styler); err != nil {
		return err
	}
	if res.NextCursor != "" {
		hint := fmt.Sprintf("More results available. To fetch the next page:\n  %s", nextPageCmd(res.NextCursor))
		_, err := fmt.Fprintf(w, "\n%s\n", s.Dim.Render(hint))
		return err
	}
	return nil
}
