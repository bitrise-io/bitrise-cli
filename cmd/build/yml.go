package build

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalyml "github.com/bitrise-io/bitrise-cli/internal/yml"
)

func newYMLCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "yml BUILD_ID",
		Short: "Print the bitrise.yml a specific build ran with",
		Long: `Print the bitrise.yml configuration that a specific build ran with.

This is a shortcut for 'bitrise-cli yml get --build BUILD_ID'.

Required:
  --app ID      app ID (or BITRISE_APP_ID env var)`,
		Example: `  bitrise-cli build yml abc123 --app my-app-id
  bitrise-cli build yml abc123 --app my-app-id --output json`,
		Args: cmdutil.RequireArgs("BUILD_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}
			appSlug, err := cmdutil.ResolveAndLookupAppSlug(cmd, client)
			if err != nil {
				return err
			}
			buildSlug := args[0]
			format := cmdutil.ResolveFormat(cmd)
			svc := internalyml.NewService(client)
			result, err := svc.Get(cmd.Context(), appSlug, buildSlug)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, result, renderBuildYML)
		},
	}
}

func renderBuildYML(w io.Writer, r internalyml.GetResult) error {
	_, err := fmt.Fprint(w, r.Content)
	return err
}
