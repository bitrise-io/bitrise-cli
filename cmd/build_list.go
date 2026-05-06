package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func newBuildListCmd() *cobra.Command {
	var (
		branch   string
		workflow string
		status   string
		limit    int
		cursor   string
	)

	c := &cobra.Command{
		Use:   "list",
		Short: "List builds for an app",
		Long: `List builds for an app, newest first.

Required flags:
  --app SLUG         (or BITRISE_APP_SLUG env var)

Optional filters:
  --branch BRANCH
  --workflow ID
  --status STATUS    one of: success, failed, in-progress, aborted

Pagination:
  --limit N
  --cursor TOKEN     opaque token from a previous page's "next_cursor"`,
		Example: `  bitrise-cli build list --app my-app-slug
  bitrise-cli build list --app my-app-slug --branch main --status failed
  bitrise-cli build list --app my-app-slug --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			appSlug, err := resolveAppSlug(cmd)
			if err != nil {
				return err
			}
			format := resolveFormat(cmd)

			svc := build.NewService()
			res, err := svc.List(cmd.Context(), build.ListOptions{
				AppSlug:  appSlug,
				Branch:   branch,
				Workflow: workflow,
				Status:   status,
				Limit:    limit,
				Cursor:   cursor,
			})
			if err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), format, res, renderBuildListText)
		},
	}

	c.Flags().String(flagApp, "", "app slug, alias: --project (or set BITRISE_APP_SLUG)")
	c.Flags().StringVar(&branch, "branch", "", "filter by branch")
	c.Flags().StringVar(&workflow, "workflow", "", "filter by workflow ID")
	c.Flags().StringVar(&status, "status", "", "filter by status (success, failed, in-progress, aborted)")
	c.Flags().IntVar(&limit, "limit", 0, "max items per page (server default if 0)")
	c.Flags().StringVar(&cursor, "cursor", "", "pagination cursor from a previous response")
	addAppProjectAlias(c)

	return c
}

func renderBuildListText(w io.Writer, res build.ListResult) error {
	if len(res.Items) == 0 {
		fmt.Fprintln(w, "No builds found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NUMBER\tSTATUS\tBRANCH\tWORKFLOW\tTRIGGERED\tSLUG")
	for _, b := range res.Items {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n",
			b.BuildNumber, b.Status, b.Branch, b.Workflow,
			b.TriggeredAt.Format("2006-01-02 15:04"), b.Slug,
		)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if res.NextCursor != "" {
		fmt.Fprintf(w, "\nMore results available — pass --cursor %s\n", res.NextCursor)
	}
	return nil
}
