package session

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

type notificationsResult struct {
	Items []internalrde.SessionNotification `json:"items"`
}

func newNotificationsCmd() *cobra.Command {
	var (
		since  string
		before string
		limit  int
		order  string
	)
	c := &cobra.Command{
		Use:   "notifications SESSION_ID",
		Short: "List notifications emitted by a session",
		Long: `List notifications a session has emitted (agent stop, permission prompt,
idle, …). Use --since with the timestamp of the newest notification you've
seen to poll for new events incrementally.`,
		Example: `  bitrise-cli rde session notifications SESSION_ID
  bitrise-cli rde session notifications SESSION_ID --since 2026-05-27T10:00:00Z --limit 100
  bitrise-cli rde session notifications SESSION_ID --order asc`,
		Args: cmdutil.RequireArgs("SESSION_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			if since != "" {
				if _, err := time.Parse(time.RFC3339, since); err != nil {
					return fmt.Errorf("--since: %w", err)
				}
			}
			if before != "" {
				if _, err := time.Parse(time.RFC3339, before); err != nil {
					return fmt.Errorf("--before: %w", err)
				}
			}
			switch order {
			case "", "asc", "desc":
			default:
				return fmt.Errorf("--order must be asc or desc")
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			items, err := internalrde.NewService(client).ListSessionNotifications(cmd.Context(), workspaceID, args[0], internalrde.ListSessionNotificationsOptions{
				CreatedBefore: before,
				CreatedAfter:  since,
				Limit:         limit,
				Order:         order,
			})
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, notificationsResult{Items: items}, renderNotifications)
		},
	}
	c.Flags().StringVar(&since, "since", "", "only notifications created after this RFC3339 timestamp (exclusive)")
	c.Flags().StringVar(&before, "before", "", "only notifications created before this RFC3339 timestamp (exclusive)")
	c.Flags().IntVar(&limit, "limit", 0, "max notifications to return (server default 50, max 100)")
	c.Flags().StringVar(&order, "order", "", "sort order: asc (oldest first) or desc (newest first); server default is desc")

	_ = c.RegisterFlagCompletionFunc("order", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"asc\toldest first", "desc\tnewest first (default)"}, cobra.ShellCompDirectiveNoFileComp
	})
	return c
}

func renderNotifications(w io.Writer, r notificationsResult) error {
	if len(r.Items) == 0 {
		_, err := fmt.Fprintln(w, "No notifications.")
		return err
	}
	s := style.New(w)
	headers := []string{"CREATED", "TYPE", "TITLE", "ID"}
	rows := make([][]string, 0, len(r.Items))
	for _, n := range r.Items {
		created := ""
		if n.CreatedAt != nil {
			created = n.CreatedAt.UTC().Format("2006-01-02 15:04:05")
		}
		rows = append(rows, []string{created, n.Type, n.Title, n.ID})
	}
	const colID = 3
	styler := func(_, col int, content string) string {
		if col == colID {
			return s.Slug.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}
