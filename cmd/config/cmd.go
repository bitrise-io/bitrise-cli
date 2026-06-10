package config

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalconfig "github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

// NewCmd returns the `bitrise-cli config` parent command and its subcommands.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration (defaults persisted to a YAML file)",
		Long: fmt.Sprintf(`Manage persistent CLI configuration.

Storage:
  Global file: YAML at $XDG_CONFIG_HOME/bitrise/config.yaml
               (falls back to ~/.config/bitrise/config.yaml).
  Per-dir:     .bitrise-cli.yml in the current directory or any ancestor.
               Useful for per-project app_id pinning.

Precedence at runtime:
  flag > env > per-directory file > global file > built-in default

Recognized keys:
  %s

Environment overrides for the same values:
  %s, %s, %s, %s, %s, %s

Note: 'set'/'unset' modify only the global file. Per-directory files must be
edited by hand.

To manage your access token, use 'bitrise-cli auth login/logout/status'.`,
			strings.Join(internalconfig.Keys, ", "),
			internalconfig.EnvOutput, internalconfig.EnvAppSlug, internalconfig.EnvToken,
			internalconfig.EnvAPIBaseURL, internalconfig.EnvWebBaseURL, internalconfig.EnvTheme,
		),
	}
	c.AddCommand(
		newPathCmd(),
		newListCmd(),
		newGetCmd(),
		newSetCmd(),
		newUnsetCmd(),
	)
	return c
}

func newPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the absolute path of the config file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := internalconfig.Path()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), p)
			return err
		},
	}
}

// configList is the JSON shape of `bitrise-cli config list`.
type configList struct {
	Output     string `json:"output,omitempty"`
	AppSlug    string `json:"app_id,omitempty"`
	OrgSlug    string `json:"default_workspace_id,omitempty"`
	APIBaseURL string `json:"api_base_url,omitempty"`
	WebBaseURL string `json:"web_base_url,omitempty"`
	Theme      string `json:"theme,omitempty"`
	Path       string `json:"path"`
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the current config-file values",
		Long: `List the values currently saved in the config file.

Env-var overrides are NOT shown by this command — they only apply at runtime
to other bitrise-cli commands.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := internalconfig.Load()
			if err != nil {
				return err
			}
			p, err := internalconfig.Path()
			if err != nil {
				return err
			}
			v := configList{
				Output:     cfg.Output,
				AppSlug:    cfg.AppSlug,
				OrgSlug:    cfg.OrgSlug,
				APIBaseURL: cfg.APIBaseURL,
				WebBaseURL: cfg.WebBaseURL,
				Theme:      cfg.Theme,
				Path:       p,
			}
			return output.Render(cmd.OutOrStdout(), cmdutil.ResolveFormat(cmd), v, renderListHuman)
		},
	}
}

func renderListHuman(w io.Writer, v configList) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string {
		return s.Label.Render(fmt.Sprintf("%-28s", label))
	}
	value := func(v string) string {
		if v == "" {
			return s.Dim.Render("(unset)")
		}
		return v
	}
	ew.F("%s%s\n\n", lbl("Path:"), s.Dim.Render(v.Path))
	ew.F("%s%s\n", lbl(internalconfig.KeyOutput+":"), value(v.Output))
	ew.F("%s%s\n", lbl(internalconfig.KeyAppSlug+":"), value(v.AppSlug))
	ew.F("%s%s\n", lbl(internalconfig.KeyOrgSlug+":"), value(v.OrgSlug))
	ew.F("%s%s\n", lbl(internalconfig.KeyAPIBaseURL+":"), value(v.APIBaseURL))
	ew.F("%s%s\n", lbl(internalconfig.KeyWebBaseURL+":"), value(v.WebBaseURL))
	ew.F("%s%s\n", lbl(internalconfig.KeyTheme+":"), value(v.Theme))
	return ew.Err
}

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "get KEY",
		Short:     "Print the value of a single config key (raw, unmasked)",
		ValidArgs: internalconfig.Keys,
		Long: fmt.Sprintf(`Print the raw value of one config key.

Valid keys: %s`,
			strings.Join(internalconfig.Keys, ", "),
		),
		Args: cmdutil.RequireArgs("KEY"),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := internalconfig.Load()
			if err != nil {
				return err
			}
			v, err := cfg.Get(args[0])
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), v)
			return err
		},
	}
}

func newSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "set KEY VALUE",
		Short:     "Set a config key and save the file",
		ValidArgs: internalconfig.Keys,
		Long: fmt.Sprintf(`Set a config key and save the file.

Valid keys: %s

The value is validated before being saved (e.g. "output" must be human or json,
"api_base_url" and "web_base_url" must be valid URLs). The file is written with
0600 permissions.

If VALUE is "-", the value is read from stdin (trailing newline trimmed).`,
			strings.Join(internalconfig.Keys, ", "),
		),
		Example: `  bitrise-cli config set output json
  bitrise-cli config set app_id 5db8b1d8-cae8-4cea-b943-ddc8f48e5e7c`,
		Args: cmdutil.RequireArgs("KEY", "VALUE"),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]
			if value == "-" {
				v, err := readStdinValue(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("read value from stdin: %w", err)
				}
				value = v
			}
			cfg, err := internalconfig.Load()
			if err != nil {
				return err
			}
			if err := cfg.Set(key, value); err != nil {
				return err
			}
			if err := internalconfig.Save(cfg); err != nil {
				return err
			}
			if !cmdutil.IsQuiet(cmd) {
				if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Saved %s\n", key); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func readStdinValue(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	s := string(data)
	s = strings.TrimSuffix(s, "\n")
	s = strings.TrimSuffix(s, "\r")
	return s, nil
}

func newUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "unset KEY",
		Short:     "Remove a config key and save the file",
		ValidArgs: internalconfig.Keys,
		Long: fmt.Sprintf(`Remove a config key and save the file.

Valid keys: %s`, strings.Join(internalconfig.Keys, ", ")),
		Args: cmdutil.RequireArgs("KEY"),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := internalconfig.Load()
			if err != nil {
				return err
			}
			if err := cfg.Unset(args[0]); err != nil {
				return err
			}
			if err := internalconfig.Save(cfg); err != nil {
				return err
			}
			if !cmdutil.IsQuiet(cmd) {
				if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Cleared %s\n", args[0]); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
