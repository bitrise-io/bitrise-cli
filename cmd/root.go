// Package cmd is the cobra-based presentation layer for the Bitrise CLI.
//
// All cobra wiring (flags, help text, subcommand registration) lives here.
// The cmd handlers parse flags, call into the service layer (internal/...),
// and format the result via internal/output. They contain no business logic.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/bitrise-io/bitrise-cli/internal/auth"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

const (
	flagOutput = "output"
	flagApp    = "app"
	flagQuiet  = "quiet"
)

// quiet is set by the persistent --quiet/-q flag. Commands check it before
// emitting non-error diagnostics ("Saved output", "Cleared token", etc.).
// Errors and primary data output ignore this flag.
var quiet bool

// rootCmd is the `bitrise-cli` entrypoint. The recommended shorter alias is
// `br` — install it as a symlink or shell alias where the name is free.
var rootCmd = &cobra.Command{
	Use:     "bitrise-cli",
	Short:   "Bitrise platform CLI",
	Version: versionString(),
	Long: `bitrise-cli is the Bitrise platform CLI — manage builds, apps, and pipelines from your terminal.

Tip:
  Install a "br" alias for less typing where the name is free, e.g.
    ln -s "$(command -v bitrise-cli)" /usr/local/bin/br
  or as a shell alias:
    alias br=bitrise-cli

Output formats:
  --output human  human-readable, default (tables and key/value lines)
  --output json   machine-readable; the schema is part of the CLI's stable contract

Configuration (precedence: flag > env > per-dir > global > built-in default):
  Global file:   $XDG_CONFIG_HOME/bitrise/config.yaml (or ~/.config/bitrise/config.yaml)
  Per-dir file:  .bitrise-cli.yml in the current directory or any ancestor
  Manage with:   bitrise-cli config set <key> <value>   (run "bitrise-cli config" for details)
  Env overrides: BITRISE_TOKEN, BITRISE_APP_SLUG, BITRISE_OUTPUT, BITRISE_API_BASE_URL

Tip for automation / agents:
  Pass --output json on every command — or run "bitrise-cli config set output json"
  once — to get parseable output. Errors and data are written to stdout in JSON
  mode; only diagnostic messages go to stderr.`,
	SilenceUsage:      true,
	SilenceErrors:     false,
	PersistentPreRunE: persistentPreRun,
}

// Execute runs the root command. main.go is the only caller.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Flag default is empty so we can distinguish "user didn't pass --output"
	// from "user passed --output human". The help text shows the effective default.
	rootCmd.PersistentFlags().StringP(flagOutput, "o", "", `output format: human|json (default "human")`)
	rootCmd.PersistentFlags().BoolVarP(&quiet, flagQuiet, "q", false, "suppress non-error diagnostic messages")
	rootCmd.AddCommand(newBuildCmd())
	rootCmd.AddCommand(newAppCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newVersionCmd())
}

// persistentPreRun loads global config, per-directory config, and auth.yaml,
// merges them with env + flags, and stores the Resolved settings on
// cmd.Context() so subcommand handlers can read them.
func persistentPreRun(cmd *cobra.Command, _ []string) error {
	globalCfg, err := config.Load()
	if err != nil {
		return err
	}
	dirCfg, _, err := config.LoadDir()
	if err != nil {
		return err
	}
	authData, err := auth.Load()
	if err != nil {
		return err
	}
	flagOut, _ := cmd.Flags().GetString(flagOutput)
	r, err := config.Resolve(globalCfg, dirCfg, authData, flagOut)
	if err != nil {
		return err
	}
	cmd.SetContext(config.WithResolved(cmd.Context(), r))
	return nil
}

// resolvedFromCmd is a tiny convenience wrapper. Returns the Resolved
// installed by persistentPreRun.
func resolvedFromCmd(cmd *cobra.Command) config.Resolved {
	return config.FromContext(cmd.Context())
}

// resolveFormat returns the resolved output format. Validation already
// happened in persistentPreRun, so no error is possible here.
func resolveFormat(cmd *cobra.Command) output.Format {
	return resolvedFromCmd(cmd).Output
}

// resolveAppSlug returns the app slug to operate on, layering --app over
// the env/file value carried in Resolved.
func resolveAppSlug(cmd *cobra.Command) (string, error) {
	if v, _ := cmd.Flags().GetString(flagApp); v != "" {
		return v, nil
	}
	if v := resolvedFromCmd(cmd).AppSlug; v != "" {
		return v, nil
	}
	return "", appSlugRequiredErr("--app")
}

// resolveAppSlugArg returns the positional APP_SLUG argument (args[0]),
// layering it over the env/file value. Used by `app view` and
// `app workflow list` where the app is the subject of the command.
func resolveAppSlugArg(cmd *cobra.Command, args []string) (string, error) {
	if len(args) >= 1 && args[0] != "" {
		return args[0], nil
	}
	if v := resolvedFromCmd(cmd).AppSlug; v != "" {
		return v, nil
	}
	return "", appSlugRequiredErr("APP_SLUG positional argument")
}

func appSlugRequiredErr(via string) error {
	return fmt.Errorf("%s is required (or set %s, or run 'bitrise-cli config set %s <slug>')",
		via, config.EnvAppSlug, config.KeyAppSlug)
}

// addAppProjectAlias registers the --project flag as a parse-time synonym
// for --app on the given command's local flag set. Users may type either name;
// only --app appears in --help (alias is documented in the flag description).
func addAppProjectAlias(c *cobra.Command) {
	c.Flags().SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "project" {
			return pflag.NormalizedName(flagApp)
		}
		return pflag.NormalizedName(name)
	})
}
