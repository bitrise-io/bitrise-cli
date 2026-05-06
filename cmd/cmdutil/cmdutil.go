// Package cmdutil holds helpers shared across the cmd sub-packages.
package cmdutil

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

const (
	FlagOutput = "output"
	FlagApp    = "app"
	FlagQuiet  = "quiet"
)

// IsQuiet reports whether the persistent --quiet flag was set.
func IsQuiet(cmd *cobra.Command) bool {
	q, _ := cmd.Root().PersistentFlags().GetBool(FlagQuiet)
	return q
}

// ResolveFormat returns the resolved output format from context.
func ResolveFormat(cmd *cobra.Command) output.Format {
	return config.FromContext(cmd.Context()).Output
}

// ResolveAppSlug returns the app slug, preferring --app then Resolved.
func ResolveAppSlug(cmd *cobra.Command) (string, error) {
	if v, _ := cmd.Flags().GetString(FlagApp); v != "" {
		return v, nil
	}
	if v := config.FromContext(cmd.Context()).AppSlug; v != "" {
		return v, nil
	}
	return "", AppSlugRequiredErr("--app")
}

// ResolveAppSlugArg returns the positional APP_SLUG argument, falling back to Resolved.
func ResolveAppSlugArg(cmd *cobra.Command, args []string) (string, error) {
	if len(args) >= 1 && args[0] != "" {
		return args[0], nil
	}
	if v := config.FromContext(cmd.Context()).AppSlug; v != "" {
		return v, nil
	}
	return "", AppSlugRequiredErr("APP_SLUG positional argument")
}

// AppSlugRequiredErr returns the standard missing-app-slug error.
func AppSlugRequiredErr(via string) error {
	return fmt.Errorf("%s is required (or set %s, or run 'bitrise-cli config set %s <slug>')",
		via, config.EnvAppSlug, config.KeyAppSlug)
}

// AddAppProjectAlias registers --project as a parse-time synonym for --app.
func AddAppProjectAlias(c *cobra.Command) {
	c.Flags().SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "project" {
			return pflag.NormalizedName(FlagApp)
		}
		return pflag.NormalizedName(name)
	})
}

// ErrWriter wraps an io.Writer and captures the first write error so callers
// can chain writes and check once at the end.
type ErrWriter struct {
	w   io.Writer
	Err error
}

// NewErrWriter returns an ErrWriter backed by w.
func NewErrWriter(w io.Writer) *ErrWriter { return &ErrWriter{w: w} }

// F writes a formatted string, skipping if a previous write already failed.
func (ew *ErrWriter) F(format string, a ...any) {
	if ew.Err != nil {
		return
	}
	_, ew.Err = fmt.Fprintf(ew.w, format, a...)
}

// Ln writes args followed by a newline, skipping if a previous write failed.
func (ew *ErrWriter) Ln(a ...any) {
	if ew.Err != nil {
		return
	}
	_, ew.Err = fmt.Fprintln(ew.w, a...)
}
