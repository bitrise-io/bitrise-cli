// Package image wires `bitrise-cli rde image` subcommands.
package image

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
	Items []internalrde.Image `json:"items"`
}

// NewCmd returns the `rde image` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "image",
		Short: "List machine images available to the workspace",
		RunE:  cmdutil.DelegateToList,
	}
	c.AddCommand(newListCmd())
	return c
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List machine images",
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
			items, err := internalrde.NewService(client).ListImages(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, listResult{Items: items}, renderList)
		},
	}
}

func renderList(w io.Writer, res listResult) error {
	if len(res.Items) == 0 {
		_, err := fmt.Fprintln(w, "No images found.")
		return err
	}
	s := style.New(w)
	headers := []string{"NAME", "ID"}
	seen := make(map[string]struct{}, len(res.Items))
	rows := make([][]string, 0, len(res.Items))
	for _, im := range res.Items {
		if _, ok := seen[im.Name]; ok {
			continue
		}
		seen[im.Name] = struct{}{}
		rows = append(rows, []string{im.Name, im.ID})
	}
	styler := func(_, col int, content string) string {
		if col == 1 {
			return s.Slug.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}
