package template

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

type listResult struct {
	Items []internalrde.Template `json:"items"`
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List RDE templates in the workspace",
		Example: `  bitrise-cli rde template list
  bitrise-cli rde template list --output json | jq '.items[].id'`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			items, err := internalrde.NewService(client).ListTemplates(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, listResult{Items: items}, renderList)
		},
	}
}

func renderList(w io.Writer, res listResult) error {
	if len(res.Items) == 0 {
		_, err := fmt.Fprintln(w, "No templates found.")
		return err
	}
	s := style.New(w)
	headers := []string{"NAME", "STACK", "MACHINE", "OWNER", "ID"}
	rows := make([][]string, 0, len(res.Items))
	for _, t := range res.Items {
		rows = append(rows, []string{t.Name, t.StackID, t.MachineType, t.CreatedByEmail, t.ID})
	}
	const colID = 4
	styler := func(_, col int, content string) string {
		if col == colID {
			return s.Slug.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}
