package app

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalapp "github.com/bitrise-io/bitrise-cli/internal/app"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

func newListCmd() *cobra.Command {
	var (
		limit       int
		cursor      string
		sortBy      string
		title       string
		projectType string
		fetchAll    bool
	)

	c := &cobra.Command{
		Use:   "list",
		Short: "List apps the authenticated user can access",
		Long: `List all apps (projects) the authenticated user can access.

Filters:
  --title TITLE              filter apps by title
  --project-type TYPE        e.g. ios, android
  --sort-by FIELD            ordering accepted by the API (e.g. created_at, last_build_at)

Pagination:
  --limit N                  max items per page (server default if 0)
  --cursor TOKEN             opaque token from a previous page's next_cursor
  --all                      fetch all pages automatically

In JSON mode (--output json), the next_cursor field holds the cursor value for scripting:
  bitrise-cli app list --output json | jq -r '.next_cursor'`,
		Example: `  bitrise-cli app list
  bitrise-cli app list --all
  bitrise-cli app list --output json | jq -r '.next_cursor'
  bitrise-cli app list --project-type ios --limit 100`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if fetchAll && cursor != "" {
				return fmt.Errorf("--all and --cursor cannot be used together")
			}

			format := cmdutil.ResolveFormat(cmd)

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}
			svc := internalapp.NewService(client)

			makeOpts := func(cur string) internalapp.ListOptions {
				return internalapp.ListOptions{
					Limit:       limit,
					Cursor:      cur,
					SortBy:      sortBy,
					Title:       title,
					ProjectType: projectType,
				}
			}

			var res internalapp.AppsResult
			if fetchAll {
				var allItems []internalapp.App
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
				res = internalapp.AppsResult{Items: allItems}
			} else {
				res, err = svc.List(cmd.Context(), makeOpts(cursor))
				if err != nil {
					return err
				}
			}

			nextPageCmd := func(nextCursor string) string {
				parts := []string{"bitrise-cli app list"}
				if cmd.Flags().Changed("title") {
					parts = append(parts, "--title", title)
				}
				if cmd.Flags().Changed("project-type") {
					parts = append(parts, "--project-type", projectType)
				}
				if cmd.Flags().Changed("sort-by") {
					parts = append(parts, "--sort-by", sortBy)
				}
				if cmd.Flags().Changed("limit") {
					parts = append(parts, "--limit", strconv.Itoa(limit))
				}
				parts = append(parts, "--cursor", nextCursor)
				return strings.Join(parts, " ")
			}

			render := func(w io.Writer, r internalapp.AppsResult) error {
				return renderListText(w, r, nextPageCmd)
			}
			return output.Render(cmd.OutOrStdout(), format, res, render)
		},
	}

	c.Flags().IntVar(&limit, "limit", 0, "max items per page (server default if 0)")
	c.Flags().StringVar(&cursor, "cursor", "", "pagination cursor from a previous response")
	c.Flags().BoolVar(&fetchAll, "all", false, "fetch all pages automatically")
	c.Flags().StringVar(&sortBy, "sort-by", "", "ordering accepted by the API (e.g. created_at, last_build_at)")
	c.Flags().StringVar(&title, "title", "", "filter apps by title")
	c.Flags().StringVar(&projectType, "project-type", "", "filter by project type (ios, android, ...)")

	_ = c.RegisterFlagCompletionFunc("sort-by", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"created_at\tsort by creation date", "last_build_at\tsort by last build date"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = c.RegisterFlagCompletionFunc("project-type", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"ios", "android", "flutter", "react-native", "xamarin", "cordova", "ionic", "other"}, cobra.ShellCompDirectiveNoFileComp
	})

	return c
}

func renderListText(w io.Writer, res internalapp.AppsResult, nextPageCmd func(string) string) error {
	if len(res.Items) == 0 {
		_, err := fmt.Fprintln(w, "No apps found.")
		return err
	}

	s := style.New(w)
	headers := []string{"TITLE", "PROVIDER", "PROJECT_TYPE", "WORKSPACE", "DISABLED", "ID"}
	rows := make([][]string, 0, len(res.Items))
	disabled := make([]bool, 0, len(res.Items))
	for _, a := range res.Items {
		owner := a.OwnerSlug
		if a.OwnerType != "" {
			owner = fmt.Sprintf("%s/%s", a.OwnerType, a.OwnerSlug)
		}
		dis := ""
		if a.IsDisabled {
			dis = "yes"
		}
		disabled = append(disabled, a.IsDisabled)
		rows = append(rows, []string{a.Title, a.Provider, a.ProjectType, owner, dis, a.Slug})
	}
	const colSlug = 5
	styler := func(row, col int, content string) string {
		if disabled[row] {
			// Whole row dimmed when the app is disabled.
			return s.Dim.Render(content)
		}
		if col == colSlug {
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
