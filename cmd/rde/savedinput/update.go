package savedinput

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newUpdateCmd() *cobra.Command {
	var (
		value    string
		isSecret bool
	)
	c := &cobra.Command{
		Use:     "update SAVED_INPUT_ID",
		Short:   "Update a saved input's value and/or secret flag",
		Long:    `Pass --value - to read the new value from stdin.`,
		Args:    cmdutil.RequireArgs("SAVED_INPUT_ID"),
		Example: `  bitrise-cli rde saved-input update ID --value new-value`,
		RunE: func(cmd *cobra.Command, args []string) error {
			req := internalrde.UpdateSavedInputRequest{}
			if cmd.Flags().Changed("value") {
				if value == "-" {
					v, err := cmdutil.ReadSecretInput(cmd.InOrStdin(), cmd.ErrOrStderr(), "Value: ", true)
					if err != nil {
						return err
					}
					value = v
				}
				req.Value = &value
			}
			if cmd.Flags().Changed("secret") {
				b := isSecret
				req.IsSecret = &b
			}
			if req.Value == nil && req.IsSecret == nil {
				return fmt.Errorf("at least one of --value or --secret is required")
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			in, err := internalrde.NewService(client).UpdateSavedInput(cmd.Context(), args[0], req)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, in, renderDetail)
		},
	}
	c.Flags().StringVar(&value, "value", "", "new value (use '-' to read from stdin)")
	c.Flags().BoolVar(&isSecret, "secret", false, "set/unset the secret flag")
	return c
}
