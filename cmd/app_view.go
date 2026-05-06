package cmd

import (
	"fmt"
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
	fmt.Fprintf(w, "Title:        %s\n", a.Title)
	fmt.Fprintf(w, "Slug:         %s\n", a.Slug)
	fmt.Fprintf(w, "Provider:     %s\n", a.Provider)
	fmt.Fprintf(w, "Repo URL:     %s\n", a.RepoURL)
	if a.OwnerType != "" || a.OwnerSlug != "" {
		fmt.Fprintf(w, "Owner:        %s/%s\n", a.OwnerType, a.OwnerSlug)
	}
	if a.ProjectType != "" {
		fmt.Fprintf(w, "Project Type: %s\n", a.ProjectType)
	}
	if a.IsDisabled {
		fmt.Fprintln(w, "Disabled:     yes")
	}
	return nil
}
