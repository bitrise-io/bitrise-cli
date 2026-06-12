package yml

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalyml "github.com/bitrise-io/bitrise-cli/internal/yml"
)

func newGetCmd() *cobra.Command {
	var buildSlug string

	c := &cobra.Command{
		Use:   "get",
		Short: "Print the bitrise.yml stored on Bitrise",
		Long: `Print the bitrise.yml configuration stored on Bitrise for an app.

When --build is provided, prints the bitrise.yml that a specific build ran with
instead of the app's current stored configuration.

Required:
  --app ID      app ID (or BITRISE_APP_ID env var)

Optional:
  --build ID    print the yml used for this specific build`,
		Example: `  bitrise-cli yml get --app my-app-id
  bitrise-cli yml get --app my-app-id --build abc123
  bitrise-cli yml get --app my-app-id --output json
  BITRISE_APP_ID=my-app-id bitrise-cli yml get`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}
			appSlug, err := cmdutil.ResolveAndLookupAppSlug(cmd, client)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)
			svc := internalyml.NewService(client)
			result, err := svc.Get(cmd.Context(), appSlug, buildSlug)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, result, renderGetText)
		},
	}

	c.Flags().StringVar(&buildSlug, "build", "", "build ID to retrieve the yml for")
	return c
}

func renderGetText(w io.Writer, r internalyml.GetResult) error {
	_, err := fmt.Fprint(w, r.Content)
	return err
}
