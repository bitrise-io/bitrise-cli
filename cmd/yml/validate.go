package yml

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalyml "github.com/bitrise-io/bitrise-cli/internal/yml"
)

func newValidateCmd() *cobra.Command {
	var filePath string

	c := &cobra.Command{
		Use:   "validate",
		Short: "Validate a bitrise.yml file",
		Long: `Validate a bitrise.yml against the Bitrise API.

Reads from --file if provided, otherwise reads from stdin.

When --app is provided (or BITRISE_APP_SLUG is set), validation uses
app-specific settings (available stacks, machine types, license pools).
Without an app slug, only the schema is validated.

Exit codes:
  0   valid (no errors; warnings do not affect the exit code)
  1   invalid (at least one error)`,
		Example: `  bitrise-cli yml validate --file bitrise.yml
  bitrise-cli yml validate --file bitrise.yml --app my-app-slug
  cat bitrise.yml | bitrise-cli yml validate
  bitrise-cli yml validate --file bitrise.yml --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rawYAML, err := readInput(cmd.InOrStdin(), filePath)
			if err != nil {
				return fmt.Errorf("read bitrise.yml: %w", err)
			}
			if len(rawYAML) == 0 {
				return fmt.Errorf("bitrise.yml content is empty")
			}

			// --app is optional for validate; fall back to config/env but don't require it.
			appSlug := ""
			if v, _ := cmd.Flags().GetString(cmdutil.FlagApp); v != "" {
				appSlug = v
			} else if v := config.FromContext(cmd.Context()).AppSlug; v != "" {
				appSlug = v
			}

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}

			svc := internalyml.NewService(client)
			result, err := svc.Validate(cmd.Context(), string(rawYAML), appSlug)
			if err != nil {
				return err
			}

			format := cmdutil.ResolveFormat(cmd)
			if renderErr := output.Render(cmd.OutOrStdout(), format, result, renderValidateText); renderErr != nil {
				return renderErr
			}

			if !result.Valid {
				cmdutil.SilenceRootErrors(cmd)
				return fmt.Errorf("bitrise.yml is invalid")
			}
			return nil
		},
	}

	c.Flags().StringVarP(&filePath, "file", "f", "", "path to the bitrise.yml file (reads from stdin if omitted)")
	return c
}

func renderValidateText(w io.Writer, r internalyml.ValidateResult) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)

	if r.Valid {
		ew.Ln(s.Success.Render("✓") + " bitrise.yml is valid")
	} else {
		ew.Ln(s.Failure.Render("✗") + " bitrise.yml is invalid")
	}

	for _, e := range r.Errors {
		ew.F("  %s %s\n", s.Failure.Render("Error:"), e)
	}
	for _, w2 := range r.Warnings {
		ew.F("  %s %s\n", s.Warn.Render("Warning:"), w2)
	}

	return ew.Err
}
