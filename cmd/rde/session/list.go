package session

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/config"
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
	var (
		selectors []string
		mine      bool
	)
	c := &cobra.Command{
		Use:   "list",
		Short: "List RDE sessions in the workspace",
		Long: `List every RDE session the authenticated user has in the workspace.

Filter by labels with --label-selector key=value (repeatable; selectors are
exact matches and are ANDed, at most 8 per request). --mine is shorthand for
--label-selector agent=$BITRISE_AGENT_ID — it lists the sessions this agent
stamped at creation time — and errors when BITRISE_AGENT_ID isn't set.

The session list comes from the backend in arbitrary order; the CLI does
not paginate (the API doesn't paginate this endpoint either).`,
		Example: `  bitrise-cli rde session list
  bitrise-cli rde session list --workspace my-workspace
  bitrise-cli rde session list -l team=mobile -l branch=main
  bitrise-cli rde session list --mine
  bitrise-cli rde session list --output json | jq '.items[].id'`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			if mine {
				agentID := os.Getenv(config.EnvAgentID)
				if agentID == "" {
					return fmt.Errorf("--mine requires the %s environment variable (it expands to --label-selector %s=<id>)", config.EnvAgentID, agentLabelKey)
				}
				selectors = append(selectors, agentLabelKey+"="+agentID)
			}
			if err := validateLabelSelectors(selectors); err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			sessions, err := internalrde.NewService(client).ListSessions(cmd.Context(), workspaceID, selectors)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, listResult{Items: sessions}, renderSessionList)
		},
	}
	c.Flags().StringArrayVarP(&selectors, "label-selector", "l", nil, "only sessions whose labels match key=value exactly (repeatable; multiple selectors must all match)")
	c.Flags().BoolVar(&mine, "mine", false, "only sessions labeled agent=$BITRISE_AGENT_ID (shorthand for --label-selector; requires BITRISE_AGENT_ID)")
	return c
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
