package session

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

// listResult wraps the slice so --output json emits {"items": [...]}
// instead of a bare array — matches the other CLI commands' shape.
type listResult struct {
	Items []internalrde.Session `json:"items"`
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List RDE sessions in the workspace",
		Long: `List every RDE session the authenticated user has in the workspace.

The session list comes from the backend in arbitrary order; the CLI does
not paginate (the API doesn't paginate this endpoint either).`,
		Example: `  bitrise-cli rde session list
  bitrise-cli rde session list --workspace my-workspace
  bitrise-cli rde session list --output json | jq '.items[].id'`,
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
			sessions, err := internalrde.NewService(client).ListSessions(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, listResult{Items: sessions}, renderSessionList)
		},
	}
}

func renderSessionList(w io.Writer, res listResult) error {
	if len(res.Items) == 0 {
		_, err := fmt.Fprintln(w, "No sessions found.")
		return err
	}
	s := style.New(w)
	headers := []string{"NAME", "STATUS", "TEMPLATE", "CREATED", "ID"}
	rows := make([][]string, 0, len(res.Items))
	statuses := make([]string, 0, len(res.Items))
	for _, sess := range res.Items {
		statuses = append(statuses, sess.Status)
		rows = append(rows, []string{
			sess.Name,
			sess.Status,
			sess.TemplateName,
			formatTime(sess.CreatedAt),
			sess.ID,
		})
	}
	const (
		colStatus = 1
		colID     = 4
	)
	styler := func(row, col int, content string) string {
		switch col {
		case colStatus:
			return statusStyle(s, statuses[row]).Render(content)
		case colID:
			return s.Slug.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}
