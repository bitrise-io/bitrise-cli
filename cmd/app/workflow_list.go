package app

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalapp "github.com/bitrise-io/bitrise-cli/internal/app"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func newWorkflowListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list APP_SLUG",
		Short: "List workflows defined on an app",
		Long: `List the workflow IDs defined in an app's bitrise.yml.

Argument:
  APP_SLUG           the unique slug of the app.
                     Falls back to BITRISE_APP_SLUG when omitted.`,
		Example: `  bitrise-cli app workflow list stub-app-aaa
  bitrise-cli app workflow list stub-app-aaa --output json
  BITRISE_APP_SLUG=stub-app-aaa bitrise-cli app workflow list`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appSlug, err := cmdutil.ResolveAppSlugArg(cmd, args)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}
			svc := internalapp.NewService(client)
			res, err := svc.ListWorkflows(cmd.Context(), appSlug)
			if err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), format, res, renderWorkflowsText)
		},
	}
}

func renderWorkflowsText(w io.Writer, res internalapp.WorkflowsResult) error {
	ew := cmdutil.NewErrWriter(w)
	if len(res.Items) == 0 {
		ew.Ln("No workflows defined.")
		return ew.Err
	}
	for _, wf := range res.Items {
		ew.Ln(wf.ID)
	}
	return ew.Err
}
