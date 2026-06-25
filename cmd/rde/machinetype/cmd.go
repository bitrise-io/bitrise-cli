// Package machinetype wires `bitrise-cli rde machine-type` subcommands.
package machinetype

import (
	"fmt"
	"io"
	"strings"

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
		Short: "List machine types compatible with a given stack",
		RunE:  cmdutil.DelegateToList,
	}
	c.AddCommand(newListCmd())
	return c
}

func newListCmd() *cobra.Command {
	var stackID string
	c := &cobra.Command{
		Use:   "list --stack STACK_ID",
		Short: "List machine types compatible with a given stack",
		Long: `List machine types compatible with the stack given by --stack.

Each machine type is offered by one or more clusters. The cluster name is
shown only when a machine type is offered by more than one cluster for the
selected stack — pass that name as --cluster to 'rde session create' to
pin a target.`,
		Example: `  bitrise-cli rde machine-type list --stack osx-xcode-16.0.x-edge`,
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
			items, err := internalrde.NewService(client).MachineTypesForStack(cmd.Context(), workspaceID, stackID)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, listResult{Items: items}, renderList)
		},
	}
	c.Flags().StringVar(&stackID, "stack", "", "stack ID to list compatible machine types for (required)")
	_ = c.MarkFlagRequired("stack")
	return c
}

func renderList(w io.Writer, res listResult) error {
	if len(res.Items) == 0 {
		_, err := fmt.Fprintln(w, "No machine types available for that stack.")
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
	const idCol = 3 // NAME, TITLE, SPECS, ID, [CLUSTER]
	if ambiguous {
		headers = []string{"NAME", "TITLE", "SPECS", "ID", "CLUSTER"}
		for _, mt := range res.Items {
			rows = append(rows, []string{mt.Name, mt.Title, specOf(mt), mt.ID, mt.ClusterName})
		}
	} else {
		headers = []string{"NAME", "TITLE", "SPECS", "ID"}
		seen := make(map[string]struct{}, len(res.Items))
		for _, mt := range res.Items {
			if _, ok := seen[mt.Name]; ok {
				continue
			}
			seen[mt.Name] = struct{}{}
			rows = append(rows, []string{mt.Name, mt.Title, specOf(mt), mt.ID})
		}
	}
	styler := func(_, col int, content string) string {
		if col == idCol {
			return s.Slug.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}

// specOf renders the "<cpu> · <ram>" specs column from the backend's structured
// fields, or "" when none are provided.
func specOf(mt internalrde.MachineType) string {
	parts := make([]string, 0, 2)
	if mt.CPU != "" {
		parts = append(parts, mt.CPU)
	}
	if mt.RAM != "" {
		parts = append(parts, mt.RAM)
	}
	return strings.Join(parts, " · ")
}
