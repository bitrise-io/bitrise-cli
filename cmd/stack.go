package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalstack "github.com/bitrise-io/bitrise-cli/internal/stack"
)

func newStackCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "stack",
		Short: "List available stacks",
		RunE:  cmdutil.DelegateToList,
	}
	c.AddCommand(newStackListCmd())
	return c
}

func newStackListCmd() *cobra.Command {
	var workspaceSlug string

	c := &cobra.Command{
		Use:   "list",
		Short: "List available stacks and their machine configurations",
		Long: `List all available stacks with their OS, status, and version information.

When --workspace is provided, returns stacks available for that workspace,
including any custom stacks configured for it.
Without --workspace, returns globally available stacks.`,
		Example: `  bitrise-cli stack list
  bitrise-cli stack list --workspace my-workspace-slug
  bitrise-cli stack list --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format := cmdutil.ResolveFormat(cmd)

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}

			svc := internalstack.NewService(client)
			result, err := svc.List(cmd.Context(), workspaceSlug)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, result, renderStackListText)
		},
	}

	c.Flags().StringVar(&workspaceSlug, "workspace", "", "workspace slug for workspace-specific stacks (including custom stacks)")
	return c
}

func renderStackListText(w io.Writer, r internalstack.StacksResult) error {
	if len(r.Items) == 0 {
		_, err := fmt.Fprintln(w, "No stacks found.")
		return err
	}

	s := style.New(w)
	headers := []string{"ID", "TITLE", "OS", "STATUS", "REMOVAL_DATE"}
	rows := make([][]string, 0, len(r.Items))
	statuses := make([]string, 0, len(r.Items))
	for _, st := range r.Items {
		statuses = append(statuses, st.Status)
		rows = append(rows, []string{st.ID, st.Title, st.OS, st.Status, st.RemovalDate})
	}
	const colStatus = 3
	styler := func(row, col int, content string) string {
		if col == colStatus {
			switch statuses[row] {
			case "stable":
				return s.Success.Render(content)
			case "edge":
				return s.Warn.Render(content)
			case "frozen":
				return s.Dim.Render(content)
			}
		}
		if col == 0 {
			return s.Slug.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}
