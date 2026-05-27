package template

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newCreateCmd() *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a new RDE template from a JSON spec file",
		Long: `Create a new RDE template from a JSON spec file. The JSON shape matches
'rde template view --output json' (audit fields like id/created_at are
ignored), so the natural workflow is:

  bitrise-cli rde template view OTHER_TEMPLATE_ID -o json > template.json
  # edit template.json
  bitrise-cli rde template create --file template.json

Pass --file - to read the JSON from stdin.

Required fields in the JSON: name, image, machine_type.`,
		Example: `  bitrise-cli rde template create --file template.json
  cat template.json | bitrise-cli rde template create --file -`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if file == "" {
				return fmt.Errorf("--file is required")
			}
			spec, err := readTemplateSpec(cmd.InOrStdin(), file)
			if err != nil {
				return err
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
			t, err := internalrde.NewService(client).CreateTemplate(cmd.Context(), workspaceID, spec)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, t, renderDetail)
		},
	}
	c.Flags().StringVarP(&file, "file", "f", "", "path to a JSON spec file (use '-' for stdin)")
	return c
}

func newUpdateCmd() *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:   "update TEMPLATE_ID",
		Short: "Update an existing RDE template from a JSON spec file",
		Long: `Update an existing RDE template from a JSON spec file.

Only fields present in the file are sent. Array fields (template_variables,
session_inputs, feature_flags, workspace_links) replace the server's
existing list wholesale when present — to clear one, include it as [].

Round-trip workflow:

  bitrise-cli rde template view TEMPLATE_ID -o json > template.json
  # edit template.json
  bitrise-cli rde template update TEMPLATE_ID --file template.json

Pass --file - to read the JSON from stdin.`,
		Args: cmdutil.RequireArgs("TEMPLATE_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file is required")
			}
			spec, err := readTemplateSpec(cmd.InOrStdin(), file)
			if err != nil {
				return err
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
			t, err := internalrde.NewService(client).UpdateTemplate(cmd.Context(), workspaceID, args[0], spec)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, t, renderDetail)
		},
	}
	c.Flags().StringVarP(&file, "file", "f", "", "path to a JSON spec file (use '-' for stdin)")
	return c
}

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete TEMPLATE_ID",
		Short: "Delete an RDE template",
		Long: `Delete an RDE template (soft-delete server-side). Existing sessions
created from this template keep working — they reference a snapshot — but
the template can no longer be selected for new sessions.`,
		Args: cmdutil.RequireArgs("TEMPLATE_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			if err := internalrde.NewService(client).DeleteTemplate(cmd.Context(), workspaceID, args[0]); err != nil {
				return err
			}
			if !cmdutil.IsQuiet(cmd) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Deleted template %s\n", args[0])
			}
			return nil
		},
	}
}

// readTemplateSpec reads a TemplateSpec from path. "-" reads from stdin.
// Extra fields (id, created_at, etc.) are ignored — encoding/json drops
// them naturally — so view-output can be piped straight back in.
func readTemplateSpec(stdin io.Reader, path string) (internalrde.TemplateSpec, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path) //nolint:gosec // path comes from user --file flag
	}
	if err != nil {
		return internalrde.TemplateSpec{}, fmt.Errorf("read template spec: %w", err)
	}
	var spec internalrde.TemplateSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return internalrde.TemplateSpec{}, fmt.Errorf("parse template spec: %w", err)
	}
	return spec, nil
}
