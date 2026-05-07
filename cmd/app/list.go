package app

import (
	"fmt"
	"io"

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
	)

	c := &cobra.Command{
		Use:   "list",
		Short: "List apps the authenticated user can access",
		Long: `List all apps (projects) the authenticated user can access.

Filters:
  --title TITLE              filter apps whose title contains the given string (case-insensitive)
  --project-type TYPE        e.g. ios, android
  --sort-by FIELD            ordering accepted by the API (e.g. created_at, last_build_at)

Pagination:
  --limit N                  max items per page (server default if 0)
  --cursor TOKEN             opaque token from a previous page's "next_cursor"`,
		Example: `  bitrise-cli app list
  bitrise-cli app list --output json
  bitrise-cli app list --project-type ios --limit 100`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format := cmdutil.ResolveFormat(cmd)

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}
			svc := internalapp.NewService(client)
			res, err := svc.List(cmd.Context(), internalapp.ListOptions{
				Limit:       limit,
				Cursor:      cursor,
				SortBy:      sortBy,
				Title:       title,
				ProjectType: projectType,
			})
			if err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), format, res, renderListText)
		},
	}

	c.Flags().IntVar(&limit, "limit", 0, "max items per page (server default if 0)")
	c.Flags().StringVar(&cursor, "cursor", "", "pagination cursor from a previous response")
	c.Flags().StringVar(&sortBy, "sort-by", "", "ordering accepted by the API (e.g. created_at, last_build_at)")
	c.Flags().StringVar(&title, "title", "", "filter apps whose title contains the given string (case-insensitive)")
	c.Flags().StringVar(&projectType, "project-type", "", "filter by project type (ios, android, ...)")

	_ = c.RegisterFlagCompletionFunc("sort-by", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"created_at\tsort by creation date", "last_build_at\tsort by last build date"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = c.RegisterFlagCompletionFunc("project-type", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"ios", "android", "flutter", "react-native", "xamarin", "cordova", "ionic", "other"}, cobra.ShellCompDirectiveNoFileComp
	})

	return c
}

func renderListText(w io.Writer, res internalapp.AppsResult) error {
	if len(res.Items) == 0 {
		_, err := fmt.Fprintln(w, "No apps found.")
		return err
	}

	s := style.New(w)
	headers := []string{"TITLE", "PROVIDER", "PROJECT", "OWNER", "DISABLED", "SLUG"}
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
		_, err := fmt.Fprintf(w, "\n%s\n", s.Dim.Render("More results available — pass --cursor "+res.NextCursor))
		return err
	}
	return nil
}
