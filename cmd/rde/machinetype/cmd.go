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
		Short: "List machine types compatible with a given image",
		RunE:  cmdutil.DelegateToList,
	}
	c.AddCommand(newListCmd())
	return c
}

func newListCmd() *cobra.Command {
	var imageName string
	c := &cobra.Command{
		Use:   "list --image NAME",
		Short: "List machine types compatible with a given image",
		Long: `List machine types compatible with the image given by --image.

Each machine type is offered by one or more clusters. The cluster name is
shown only when a machine type is offered by more than one cluster for the
selected image — pass that name as --cluster to 'rde session create' to
pin a target.`,
		Example: `  bitrise-cli rde machine-type list --image osx-xcode-edge`,
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
			items, err := internalrde.NewService(client).MachineTypesForImage(cmd.Context(), workspaceID, imageName)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, listResult{Items: items}, renderList)
		},
	}
	c.Flags().StringVar(&imageName, "image", "", "image name to list compatible machine types for (required)")
	_ = c.MarkFlagRequired("image")
	return c
}

func renderList(w io.Writer, res listResult) error {
	if len(res.Items) == 0 {
		_, err := fmt.Fprintln(w, "No machine types available for that image.")
		return err
	}
	clustersPerName := make(map[string]map[string]struct{}, len(res.Items))
	for _, mt := range res.Items {
		set, ok := clustersPerName[mt.Name]
		if !ok {
			set = make(map[string]struct{})
			clustersPerName[mt.Name] = set
		}
		set[mt.ClusterName] = struct{}{}
	}
	ambiguous := false
	for _, set := range clustersPerName {
		if len(set) > 1 {
			ambiguous = true
			break
		}
	}

	s := style.New(w)
	var headers []string
	var rows [][]string
	if ambiguous {
		headers = []string{"NAME", "ID", "CLUSTER"}
		for _, mt := range res.Items {
			rows = append(rows, []string{mt.Name, mt.ID, mt.ClusterName})
		}
	} else {
		headers = []string{"NAME", "ID"}
		seen := make(map[string]struct{}, len(res.Items))
		for _, mt := range res.Items {
			if _, ok := seen[mt.Name]; ok {
				continue
			}
			seen[mt.Name] = struct{}{}
			rows = append(rows, []string{mt.Name, mt.ID})
		}
	}
	styler := func(_, col int, content string) string {
		if col == 1 {
			return s.Slug.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}
