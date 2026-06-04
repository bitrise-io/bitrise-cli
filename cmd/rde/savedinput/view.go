package savedinput

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view SAVED_INPUT_ID",
		Short: "Show details of a single saved input",
		Args:  cmdutil.RequireArgs("SAVED_INPUT_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			in, err := internalrde.NewService(client).GetSavedInput(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, in, renderDetail)
		},
	}
}

func renderDetail(w io.Writer, in internalrde.SavedInput) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string { return s.Label.Render(fmt.Sprintf("%-12s", label)) }

	ew.F("%s%s\n", lbl("Key:"), in.Key)
	ew.F("%s%s\n", lbl("ID:"), s.Slug.Render(in.ID))
	if in.IsSecret {
		ew.F("%s%s\n", lbl("Value:"), s.Dim.Render("(hidden)"))
		ew.F("%s%s\n", lbl("Secret:"), "yes")
	} else {
		ew.F("%s%s\n", lbl("Value:"), in.Value)
	}
	if in.UpdatedAt != nil {
		ew.F("%s%s\n", lbl("Updated:"), in.UpdatedAt.UTC().Format("2006-01-02 15:04 UTC"))
	}
	return ew.Err
}
