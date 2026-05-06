package app

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalapp "github.com/bitrise-io/bitrise-cli/internal/app"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func newListCmd() *cobra.Command {
	var (
		limit  int
		cursor string
	)

	c := &cobra.Command{
		Use:   "list",
		Short: "List apps the authenticated user can access",
		Long: `List all apps (projects) the authenticated user can access.

Pagination:
  --limit N
  --cursor TOKEN     opaque token from a previous page's "next_cursor"`,
		Example: `  bitrise-cli app list
  bitrise-cli app list --output json
  bitrise-cli app list --limit 100`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format := cmdutil.ResolveFormat(cmd)

			svc := internalapp.NewService()
			res, err := svc.List(cmd.Context(), internalapp.ListOptions{
				Limit:  limit,
				Cursor: cursor,
			})
			if err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), format, res, renderListText)
		},
	}

	c.Flags().IntVar(&limit, "limit", 0, "max items per page (server default if 0)")
	c.Flags().StringVar(&cursor, "cursor", "", "pagination cursor from a previous response")

	return c
}

func renderListText(w io.Writer, res internalapp.AppsResult) error {
	if len(res.Items) == 0 {
		fmt.Fprintln(w, "No apps found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "TITLE\tPROVIDER\tPROJECT\tOWNER\tDISABLED\tSLUG")
	for _, a := range res.Items {
		owner := a.OwnerSlug
		if a.OwnerType != "" {
			owner = fmt.Sprintf("%s/%s", a.OwnerType, a.OwnerSlug)
		}
		disabled := ""
		if a.IsDisabled {
			disabled = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			a.Title, a.Provider, a.ProjectType, owner, disabled, a.Slug,
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
