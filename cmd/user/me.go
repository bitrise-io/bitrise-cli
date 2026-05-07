package user

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internaluser "github.com/bitrise-io/bitrise-cli/internal/user"
)

func newMeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Show the currently authenticated user",
		Long: `Show the profile of the user whose token is in use.

The token is resolved from BITRISE_TOKEN, auth.yaml, or config.yaml — run
'bitrise-cli auth status' to confirm which source is active.`,
		Example: `  bitrise-cli user me
  bitrise-cli user me --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}
			svc := internaluser.NewProfileService(client)
			profile, err := svc.Me(cmd.Context())
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), cmdutil.ResolveFormat(cmd), profile, renderMeHuman)
		},
	}
}

func renderMeHuman(w io.Writer, p internaluser.Profile) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string {
		return s.Label.Render(fmt.Sprintf("%-10s", label))
	}
	ew.F("%s%s\n", lbl("Username:"), p.Username)
	ew.F("%s%s\n", lbl("Email:"), p.Email)
	return ew.Err
}
