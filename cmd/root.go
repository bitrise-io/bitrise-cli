// Package cmd is the cobra-based presentation layer for the Bitrise CLI.
//
// All cobra wiring (flags, help text, subcommand registration) lives here.
// The cmd handlers parse flags, call into the service layer (internal/...),
// and format the result via internal/output. They contain no business logic.
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	cmdapp "github.com/bitrise-io/bitrise-cli/cmd/app"
	cmdbuild "github.com/bitrise-io/bitrise-cli/cmd/build"
	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	cmdconfig "github.com/bitrise-io/bitrise-cli/cmd/config"
	cmduser "github.com/bitrise-io/bitrise-cli/cmd/user"
	"github.com/bitrise-io/bitrise-cli/internal/auth"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

// quiet is set by the persistent --quiet/-q flag. Commands in this package
// (auth) check it directly; subpackages use cmdutil.IsQuiet(cmd).
var quiet bool

// noColor is set by the persistent --no-color flag. NO_COLOR / FORCE_COLOR
// env vars are honored automatically by the underlying termenv detection;
// this flag is the explicit override surfaced in --help.
var noColor bool

// theme is the raw --theme flag value (empty when unset). Resolved into
// a typed style.Theme by config.Resolve, then applied via style.Configure.
var theme string

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
  Env overrides: BITRISE_TOKEN, BITRISE_APP_SLUG, BITRISE_OUTPUT,
                 BITRISE_API_BASE_URL, BITRISE_WEB_BASE_URL, BITRISE_THEME

Color theme:
  --theme auto    detect terminal background via OSC 11 (default)
  --theme dark    force the dark-mode palette
  --theme light   force the light-mode palette (use on white-bg terminals)
  --theme none    disable ANSI colors entirely (same as --no-color)

Tip for automation / agents:
  Pass --output json on every command — or run "bitrise-cli config set output json"
  once — to get parseable output. Data is written to stdout; errors and diagnostics
  always go to stderr, even in JSON mode.`,
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
	rootCmd.PersistentFlags().StringP(cmdutil.FlagOutput, "o", "", `output format: human|json (default "human")`)
	rootCmd.PersistentFlags().BoolVarP(&quiet, cmdutil.FlagQuiet, "q", false, "suppress non-error diagnostic messages")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable ANSI colors (NO_COLOR env is also honored)")
	rootCmd.PersistentFlags().StringVar(&theme, cmdutil.FlagTheme, "", `color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)`)
	rootCmd.SetFlagErrorFunc(flagErrorFunc)
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.AddCommand(cmdbuild.NewCmd())
	rootCmd.AddCommand(cmdapp.NewCmd())
	rootCmd.AddCommand(cmdconfig.NewCmd())
	rootCmd.AddCommand(cmduser.NewCmd())
	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.InitDefaultCompletionCmd()
	if err := rootCmd.RegisterFlagCompletionFunc(cmdutil.FlagOutput, completeOutputFlag); err != nil {
		panic(err)
	}
	if err := rootCmd.RegisterFlagCompletionFunc(cmdutil.FlagTheme, completeThemeFlag); err != nil {
		panic(err)
	}
}

func completeOutputFlag(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return []string{"human\thuman-readable tables and key/value lines", "json\tmachine-readable JSON"}, cobra.ShellCompDirectiveNoFileComp
}

func completeThemeFlag(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return []string{
		"auto\tdetect terminal background (default)",
		"dark\tcolors tuned for dark backgrounds",
		"light\tcolors tuned for light backgrounds",
		"none\tdisable ANSI colors entirely",
	}, cobra.ShellCompDirectiveNoFileComp
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
	flagOut, _ := cmd.Flags().GetString(cmdutil.FlagOutput)
	flagTheme, _ := cmd.Flags().GetString(cmdutil.FlagTheme)
	r, err := config.Resolve(globalCfg, dirCfg, authData, flagOut, flagTheme)
	if err != nil {
		return err
	}
	// Configure must run after Resolve so the resolved theme (which folds
	// in the --theme flag, BITRISE_THEME, and the config files) is what
	// actually drives Style construction in subcommand RunE bodies.
	style.Configure(noColor, r.Theme)
	cmd.SetContext(config.WithResolved(cmd.Context(), r))
	return nil
}

// resolvedFromCmd returns the Resolved installed by persistentPreRun.
// Used by auth subcommands which live in this package.
func resolvedFromCmd(cmd *cobra.Command) config.Resolved {
	return config.FromContext(cmd.Context())
}

// resolveFormat returns the resolved output format. Validation already
// happened in persistentPreRun, so no error is possible here.
func resolveFormat(cmd *cobra.Command) output.Format {
	return resolvedFromCmd(cmd).Output
}
