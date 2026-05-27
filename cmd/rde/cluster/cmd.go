// Package cluster wires `bitrise-cli rde cluster` subcommands.
package cluster

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

type resolveResult struct {
	Items []internalrde.ClusterOption `json:"items"`
}

// NewCmd returns the `rde cluster` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "cluster",
		Short: "Resolve clusters serving a given image + machine type",
	}
	c.AddCommand(newResolveCmd())
	return c
}

func newResolveCmd() *cobra.Command {
	var (
		image       string
		machineType string
	)
	c := &cobra.Command{
		Use:   "resolve",
		Short: "Find clusters that offer both an image and a machine type",
		Long: `Find clusters that offer both the given image and machine type.

Use the cluster name returned here as --cluster when creating a session whose
template's image + machine type are available in multiple clusters.`,
		Example: `  bitrise-cli rde cluster resolve --image osx-xcode-edge --machine-type g2.mac.m2pro.4c`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if image == "" {
				return fmt.Errorf("--image is required")
			}
			if machineType == "" {
				return fmt.Errorf("--machine-type is required")
			}
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			items, err := internalrde.NewService(client).ResolveClusters(cmd.Context(), workspaceID, internalrde.ResolveClustersRequest{
				Image:       image,
				MachineType: machineType,
			})
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, resolveResult{Items: items}, renderResolve)
		},
	}
	c.Flags().StringVar(&image, "image", "", "image name (required)")
	c.Flags().StringVar(&machineType, "machine-type", "", "machine type name (required)")
	return c
}

func renderResolve(w io.Writer, res resolveResult) error {
	if len(res.Items) == 0 {
		_, err := fmt.Fprintln(w, "No clusters serve that image + machine type combination.")
		return err
	}
	s := style.New(w)
	headers := []string{"CLUSTER", "IMAGE ID", "MACHINE TYPE ID"}
	rows := make([][]string, 0, len(res.Items))
	for _, c := range res.Items {
		rows = append(rows, []string{c.ClusterName, c.ImageID, c.MachineTypeID})
	}
	styler := func(_, col int, content string) string {
		if col == 1 || col == 2 {
			return s.Slug.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}
