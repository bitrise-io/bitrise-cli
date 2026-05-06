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
	return &cobra.Command{
		Use:   "view APP_SLUG",
		Short: "Show details of a single app",
		Long: `Show details for a single app identified by its slug.

Argument:
  APP_SLUG           the unique slug of the app (visible in app URLs);
                     falls back to BITRISE_APP_SLUG when omitted`,
		Example: `  bitrise-cli app view stub-app-aaa
  bitrise-cli app view stub-app-aaa --output json
  BITRISE_APP_SLUG=stub-app-aaa bitrise-cli app view`,
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
			a, err := svc.View(cmd.Context(), appSlug)
			if err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), format, a, renderAppText)
		},
	}
}

func renderAppText(w io.Writer, a internalapp.App) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string {
		return s.Label.Render(fmt.Sprintf("%-14s", label))
	}
	ew.F("%s%s\n", lbl("Title:"), a.Title)
	ew.F("%s%s\n", lbl("Slug:"), s.Slug.Render(a.Slug))
	ew.F("%s%s\n", lbl("Provider:"), a.Provider)
	ew.F("%s%s\n", lbl("Repo URL:"), s.URL.Render(a.RepoURL))
	if a.OwnerType != "" || a.OwnerSlug != "" {
		ew.F("%s%s/%s\n", lbl("Owner:"), a.OwnerType, a.OwnerSlug)
	}
	if a.ProjectType != "" {
		ew.F("%s%s\n", lbl("Project Type:"), a.ProjectType)
	}
	if a.IsDisabled {
		ew.F("%s%s\n", lbl("Disabled:"), s.Warn.Render("yes"))
	}
	return ew.Err
}
