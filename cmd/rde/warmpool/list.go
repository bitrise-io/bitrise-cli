package warmpool

import (
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

type listResult struct {
	Items []internalrde.WarmPool `json:"items"`
}

func newListCmd() *cobra.Command {
	var (
		template string
		all      bool
	)
	c := &cobra.Command{
		Use:   "list",
		Short: "List warm pools in the workspace",
		Long: `List the workspace's warm pools. Pass --template to see only the pools
of one template (by ID or name).

--all is the read-only cost view of the same pools, the one 'rde usage'
shows; it requires the workspace's billing-data permission.

READY and WARMING are the pool's current inventory; SIZE is the pool size —
the count the backend keeps booted. STATUS is "ok", or the problem 'view' explains:
"config error" (the stored configuration no longer builds — the pool creates
nothing until it is fixed), "paused" (backing off after repeated failures)
or "error" (the last machine creation failed).`,
		Example: `  bitrise-cli rde warm-pool list
  bitrise-cli rde warm-pool list --template TEMPLATE_ID
  bitrise-cli rde warm-pool list --all --output json | jq '.items[] | {name, pool_size, ready: .status.ready}'`,
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
			svc := internalrde.NewService(client)
			templateID := ""
			if template != "" {
				if templateID, err = svc.ResolveTemplateID(cmd.Context(), workspaceID, template); err != nil {
					return err
				}
			}
			items, err := svc.ListWarmPools(cmd.Context(), workspaceID, templateID, all)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, listResult{Items: items}, renderList)
		},
	}
	c.Flags().StringVar(&template, "template", "", "only pools of this template (ID or name)")
	c.Flags().BoolVar(&all, "all", false, "the read-only cost view (requires the billing-data permission)")
	return c
}

func renderList(w io.Writer, res listResult) error {
	if len(res.Items) == 0 {
		_, err := fmt.Fprintln(w, "No warm pools found.")
		return err
	}
	s := style.New(w)
	headers := []string{"NAME", "TEMPLATE", "CREATED BY", "SIZE", "READY", "WARMING", "STATUS", "ID"}
	rows := make([][]string, 0, len(res.Items))
	for _, p := range res.Items {
		ready, warming := "", ""
		if p.Status != nil {
			ready = strconv.Itoa(p.Status.Ready)
			warming = strconv.Itoa(p.Status.Warming)
		}
		rows = append(rows, []string{
			p.Name, templateLabel(p), createdByLabel(p), strconv.Itoa(p.PoolSize),
			ready, warming, statusLabel(p.Status), p.ID,
		})
	}
	const (
		colStatus = 6
		colID     = 7
	)
	styler := func(_, col int, content string) string {
		switch col {
		case colID:
			return s.Slug.Render(content)
		case colStatus:
			if content != "ok" {
				return s.Warn.Render(content)
			}
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}

// templateLabel prefers the template's name; a pool whose template was
// deleted has only the ID left.
func templateLabel(p internalrde.WarmPool) string {
	if p.TemplateName != "" {
		return p.TemplateName
	}
	return p.TemplateID
}

// createdByLabel is the creator's email, or "workspace token" for a pool a
// Workspace API Token created (no person stands behind it).
func createdByLabel(p internalrde.WarmPool) string {
	if p.CreatedByEmail != "" {
		return p.CreatedByEmail
	}
	return "workspace token"
}

// statusLabel condenses the status into one word for the table; 'view'
// prints the messages behind it.
func statusLabel(st *internalrde.WarmPoolStatus) string {
	switch {
	case st == nil:
		return ""
	case st.ConfigError != "":
		return "config error"
	case st.PausedUntil != nil:
		return "paused"
	case st.LastError != "":
		return "error"
	}
	return "ok"
}
