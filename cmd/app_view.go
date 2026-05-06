package cmd

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/internal/app"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func newAppViewCmd() *cobra.Command {
	c := &cobra.Command{
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
			appSlug, err := resolveAppSlugArg(cmd, args)
			if err != nil {
				return err
			}
			format := resolveFormat(cmd)

			svc := app.NewService()
			a, err := svc.View(cmd.Context(), appSlug)
			if err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), format, a, renderAppText)
		},
	}
	return c
}

func renderAppText(w io.Writer, a app.App) error {
	ew := newErrWriter(w)
	ew.f("Title:        %s\n", a.Title)
	ew.f("Slug:         %s\n", a.Slug)
	ew.f("Provider:     %s\n", a.Provider)
	ew.f("Repo URL:     %s\n", a.RepoURL)
	if a.OwnerType != "" || a.OwnerSlug != "" {
		ew.f("Owner:        %s/%s\n", a.OwnerType, a.OwnerSlug)
	}
	if a.ProjectType != "" {
		ew.f("Project Type: %s\n", a.ProjectType)
	}
	if a.IsDisabled {
		ew.ln("Disabled:     yes")
	}
	return ew.err
}
