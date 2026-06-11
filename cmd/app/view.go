package app

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalapp "github.com/bitrise-io/bitrise-cli/internal/app"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

func newViewCmd() *cobra.Command {
	var web bool

	c := &cobra.Command{
		Use:   "view APP_ID",
		Short: "Show details of a single app",
		Long: `Show details for a single app identified by its ID.

Argument:
  APP_ID             the unique ID of the app (visible in app URLs);
                     falls back to BITRISE_APP_ID when omitted

Flags:
  --web              open the app page in the browser instead of printing`,
		Example: `  bitrise-cli app view stub-app-aaa
  bitrise-cli app view stub-app-aaa --output json
  bitrise-cli app view stub-app-aaa --web
  BITRISE_APP_ID=stub-app-aaa bitrise-cli app view`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}
			arg, err := cmdutil.ResolveAppSlugArg(cmd, args)
			if err != nil {
				return err
			}
			resolver := cmdutil.NewResolver(cmd, client)

			if web {
				slug, err := resolver.AppSlug(cmd.Context(), arg)
				if err != nil {
					return err
				}
				url := fmt.Sprintf("%s/app/%s", cmdutil.ResolveWebBaseURL(cmd), slug)
				if err := cmdutil.OpenBrowser(url); err != nil {
					return err
				}
				if !cmdutil.IsQuiet(cmd) {
					_, err = fmt.Fprintf(cmd.ErrOrStderr(), "Opened %s\n", url)
					return err
				}
				return nil
			}

			svc := internalapp.NewService(client)
			a, err := svc.ViewByNameOrSlug(cmd.Context(), resolver, arg)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), cmdutil.ResolveFormat(cmd), a, renderAppText)
		},
	}

	c.Flags().BoolVar(&web, cmdutil.FlagWeb, false, "open the app page in the browser")
	return c
}

func renderAppText(w io.Writer, a internalapp.App) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string {
		return s.Label.Render(fmt.Sprintf("%-16s", label))
	}
	ew.F("%s%s\n", lbl("Title:"), a.Title)
	ew.F("%s%s\n", lbl("ID:"), s.Slug.Render(a.Slug))
	ew.F("%s%s\n", lbl("Provider:"), a.Provider)
	ew.F("%s%s\n", lbl("Repo URL:"), s.URL.Render(a.RepoURL))
	if a.OwnerSlug != "" {
		ew.F("%s%s\n", lbl("Workspace:"), a.OwnerSlug)
	}
	if a.ProjectType != "" {
		ew.F("%s%s\n", lbl("Project type:"), a.ProjectType)
	}
	if a.IsDisabled {
		ew.F("%s%s\n", lbl("Disabled:"), s.Dim.Render("yes"))
	}
	return ew.Err
}
