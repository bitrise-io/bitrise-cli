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

type listResult struct {
	Items []internalrde.SavedInput `json:"items"`
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved inputs for the authenticated user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			items, err := internalrde.NewService(client).ListSavedInputs(cmd.Context())
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, listResult{Items: items}, renderList)
		},
	}
}

func renderList(w io.Writer, res listResult) error {
	if len(res.Items) == 0 {
		_, err := fmt.Fprintln(w, "No saved inputs found.")
		return err
	}
	s := style.New(w)
	headers := []string{"KEY", "SECRET", "VALUE", "ID"}
	rows := make([][]string, 0, len(res.Items))
	for _, in := range res.Items {
		secret := ""
		if in.IsSecret {
			secret = "yes"
		}
		value := in.Value
		if in.IsSecret {
			value = "(hidden)"
		}
		rows = append(rows, []string{in.Key, secret, value, in.ID})
	}
	const colID = 3
	styler := func(_, col int, content string) string {
		if col == colID {
			return s.Slug.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}
