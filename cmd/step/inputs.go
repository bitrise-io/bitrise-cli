package step

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalstep "github.com/bitrise-io/bitrise-cli/internal/step"
)

func newInputsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inputs STEP_REF",
		Short: "List inputs of a step version",
		Long: `List the inputs (and their defaults) for a given step version.

STEP_REF must include an exact version: step_id@version
For custom step sources: step_lib_source::step_id@version

Arguments:
  STEP_REF   step reference, e.g. git-clone@8.3.1`,
		Example: `  bitrise-cli step inputs git-clone@8.3.1
  bitrise-cli step inputs git-clone@8.3.1 --output json`,
		Args: cmdutil.RequireArgs("STEP_REF"),
		RunE: func(cmd *cobra.Command, args []string) error {
			format := cmdutil.ResolveFormat(cmd)

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}

			svc := internalstep.NewService(client)
			result, err := svc.Inputs(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, result, renderInputsText)
		},
	}
}

func renderInputsText(w io.Writer, r internalstep.InputsResult) error {
	if len(r.Items) == 0 {
		_, err := fmt.Fprintln(w, "No inputs found.")
		return err
	}

	s := style.New(w)
	headers := []string{"NAME", "TITLE", "DEFAULT", "REQUIRED", "SENSITIVE", "OPTIONS"}
	rows := make([][]string, 0, len(r.Items))
	required := make([]bool, 0, len(r.Items))
	for _, inp := range r.Items {
		req := ""
		if inp.IsRequired {
			req = "yes"
		}
		sens := ""
		if inp.IsSensitive {
			sens = "yes"
		}
		opts := ""
		if len(inp.ValueOptions) > 0 {
			opts = fmt.Sprintf("%v", inp.ValueOptions)
		}
		required = append(required, inp.IsRequired)
		rows = append(rows, []string{inp.Name, inp.Title, inp.DefaultValue, req, sens, opts})
	}
	styler := func(row, col int, content string) string {
		if col == 0 {
			if required[row] {
				return s.Bold.Render(content)
			}
			return s.Slug.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}
