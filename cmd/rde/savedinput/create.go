package savedinput

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newCreateCmd() *cobra.Command {
	var (
		key      string
		value    string
		isSecret bool
	)
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a new saved input",
		Long: `Create a new saved input.

Pass --value - to read the value from stdin (keeps secrets out of shell history):
  echo -n "ghp_xxx" | bitrise-cli rde saved-input create --key gh-token --value - --secret`,
		Example: `  bitrise-cli rde saved-input create --key repo-name --value my-app
  bitrise-cli rde saved-input create --key gh-token --value - --secret`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if key == "" {
				return fmt.Errorf("--key is required")
			}
			if !cmd.Flags().Changed("value") {
				return fmt.Errorf("--value is required (use --value - to read from stdin)")
			}
			if value == "-" {
				v, err := cmdutil.ReadSecretInput(cmd.InOrStdin(), cmd.ErrOrStderr(), "Value: ", true)
				if err != nil {
					return err
				}
				value = v
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			in, err := internalrde.NewService(client).CreateSavedInput(cmd.Context(), internalrde.CreateSavedInputRequest{
				Key:      key,
				Value:    value,
				IsSecret: isSecret,
			})
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, in, renderDetail)
		},
	}
	c.Flags().StringVar(&key, "key", "", "saved-input key (required)")
	c.Flags().StringVar(&value, "value", "", "value (use '-' to read from stdin)")
	c.Flags().BoolVar(&isSecret, "secret", false, "encrypt value at rest; the value will be masked in subsequent reads")
	return c
}
