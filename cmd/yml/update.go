package yml

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalyml "github.com/bitrise-io/bitrise-cli/internal/yml"
)

func newUpdateCmd() *cobra.Command {
	var filePath string

	c := &cobra.Command{
		Use:   "update",
		Short: "Upload a new bitrise.yml to Bitrise",
		Long: `Upload a new bitrise.yml configuration to Bitrise for an app.

Reads from --file if provided, otherwise reads from stdin.

Note: if the app is configured to read its bitrise.yml from the repository,
this command succeeds but the change will not affect builds.

Required:
  --app SLUG    app slug (or BITRISE_APP_SLUG env var)`,
		Example: `  bitrise-cli yml update --app my-app-slug --file bitrise.yml
  cat bitrise.yml | bitrise-cli yml update --app my-app-slug
  bitrise-cli yml update --app my-app-slug < bitrise.yml`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			appSlug, err := cmdutil.ResolveAppSlug(cmd)
			if err != nil {
				return err
			}

			rawYAML, err := readInput(cmd.InOrStdin(), filePath)
			if err != nil {
				return fmt.Errorf("read bitrise.yml: %w", err)
			}
			if len(rawYAML) == 0 {
				return fmt.Errorf("bitrise.yml content is empty")
			}

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}
			svc := internalyml.NewService(client)
			if err := svc.Update(cmd.Context(), appSlug, string(rawYAML)); err != nil {
				return err
			}

			if !cmdutil.IsQuiet(cmd) {
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "bitrise.yml updated successfully"); err != nil {
					return err
				}
			}
			return nil
		},
	}

	c.Flags().StringVarP(&filePath, "file", "f", "", "path to the bitrise.yml file (reads from stdin if omitted)")
	return c
}

// readInput reads from filePath when set, otherwise from r (stdin).
func readInput(r io.Reader, filePath string) ([]byte, error) {
	if filePath != "" && filePath != "-" {
		return os.ReadFile(filePath) //nolint:gosec // path comes from --file flag, not network input
	}
	return io.ReadAll(r)
}
