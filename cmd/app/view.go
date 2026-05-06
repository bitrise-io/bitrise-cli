package app

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalapp "github.com/bitrise-io/bitrise-cli/internal/app"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func newViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view APP_SLUG",
		Short: "Show details of a single app",
		Long: `Show details for a single app identified by its slug.

Argument:
  APP_SLUG           the unique slug of the app (visible in app URLs).
                     Falls back to BITRISE_APP_SLUG when omitted.`,
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

			svc := internalapp.NewService()
			a, err := svc.View(cmd.Context(), appSlug)
			if err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), format, a, renderAppText)
		},
	}
}

func renderAppText(w io.Writer, a internalapp.App) error {
	ew := cmdutil.NewErrWriter(w)
	ew.F("Title:        %s\n", a.Title)
	ew.F("Slug:         %s\n", a.Slug)
	ew.F("Provider:     %s\n", a.Provider)
	ew.F("Repo URL:     %s\n", a.RepoURL)
	if a.OwnerType != "" || a.OwnerSlug != "" {
		ew.F("Owner:        %s/%s\n", a.OwnerType, a.OwnerSlug)
	}
	if a.ProjectType != "" {
		ew.F("Project Type: %s\n", a.ProjectType)
	}
	if a.IsDisabled {
		ew.Ln("Disabled:     yes")
	}
	return ew.Err
}
