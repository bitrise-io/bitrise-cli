// Package machinetype wires `bitrise-cli rde machine-type` subcommands.
package machinetype

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
	Items []internalrde.MachineType `json:"items"`
}

// NewCmd returns the `rde machine-type` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "machine-type",
		Short: "List machine types available to the workspace",
		RunE:  cmdutil.DelegateToList,
	}
	c.AddCommand(newListCmd())
	return c
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List machine types",
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
			items, err := internalrde.NewService(client).ListMachineTypes(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, listResult{Items: items}, renderList)
		},
	}
}

func renderList(w io.Writer, res listResult) error {
	if len(res.Items) == 0 {
		_, err := fmt.Fprintln(w, "No machine types found.")
		return err
	}
	s := style.New(w)
	headers := []string{"ID", "NAME", "CLUSTER"}
	rows := make([][]string, 0, len(res.Items))
	for _, mt := range res.Items {
		rows = append(rows, []string{mt.ID, mt.Name, mt.ClusterName})
	}
	styler := func(_, col int, content string) string {
		if col == 0 {
			return s.Slug.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}
