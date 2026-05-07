package step

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalstep "github.com/bitrise-io/bitrise-cli/internal/step"
)

func newSearchCmd() *cobra.Command {
	var (
		categories  []string
		maintainers []string
	)

	c := &cobra.Command{
		Use:   "search QUERY",
		Short: "Find steps by name, description, or tags",
		Long: `Find steps for use in workflows or step bundles.

Returns only the latest, non-deprecated version of each matching step.

Arguments:
  QUERY   phrase to search for (e.g. "clone", "npm", "deploy")

Filters:
  --category VALUE    filter by category; may be repeated
  --maintainer VALUE  filter by maintainer; may be repeated

Valid categories:
  build, code-sign, test, deploy, notification, access-control,
  artifact-info, installer, dependency, utility

Valid maintainers:
  bitrise   official Bitrise steps
  verified  verified community steps
  community all community steps`,
		Example: `  bitrise-cli step search clone
  bitrise-cli step search deploy --category deploy --maintainer bitrise
  bitrise-cli step search npm --output json`,
		Args: cmdutil.RequireArgs("QUERY"),
		RunE: func(cmd *cobra.Command, args []string) error {
			format := cmdutil.ResolveFormat(cmd)

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}

			svc := internalstep.NewService(client)
			result, err := svc.Search(cmd.Context(), internalstep.SearchOptions{
				Query:       args[0],
				Categories:  categories,
				Maintainers: maintainers,
			})
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, result, renderSearchText)
		},
	}

	c.Flags().StringArrayVar(&categories, "category", nil, "filter by category (may be repeated)")
	c.Flags().StringArrayVar(&maintainers, "maintainer", nil, "filter by maintainer: bitrise, verified, community (may be repeated)")

	_ = c.RegisterFlagCompletionFunc("category", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"build", "code-sign", "test", "deploy", "notification",
			"access-control", "artifact-info", "installer", "dependency", "utility",
		}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = c.RegisterFlagCompletionFunc("maintainer", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"bitrise", "verified", "community"}, cobra.ShellCompDirectiveNoFileComp
	})

	return c
}

func renderSearchText(w io.Writer, r internalstep.SearchResult) error {
	if len(r.Items) == 0 {
		_, err := fmt.Fprintln(w, "No steps found.")
		return err
	}

	s := style.New(w)
	headers := []string{"STEP_REF", "TITLE", "MAINTAINER", "SUMMARY"}
	rows := make([][]string, 0, len(r.Items))
	deprecated := make([]bool, 0, len(r.Items))
	for _, step := range r.Items {
		deprecated = append(deprecated, step.IsDeprecated)
		rows = append(rows, []string{step.StepRef, step.Title, step.Maintainer, step.Summary})
	}
	styler := func(row, col int, content string) string {
		if deprecated[row] {
			return s.Dim.Render(content)
		}
		if col == 0 {
			return s.Slug.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}
