package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/auth"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

// newAuthCmd returns the `bitrise-cli auth` parent command and its subcommands.
//
// The auth surface is the recommended way to set Bitrise credentials. The
// older `config set token` flow continues to work for backward compatibility,
// but `auth login` writes to a separate auth.yaml so credentials live apart
// from preferences.
func newAuthCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "auth",
		Short: "Manage the Bitrise access token",
		Long: `Manage the Bitrise access token used for API requests.

Both Personal Access Tokens (PAT) and Workspace API Tokens (WAT) work the
same way on the wire — paste either kind here.

Storage:
  YAML file at $XDG_CONFIG_HOME/bitrise/auth.yaml (or ~/.config/bitrise/auth.yaml).
  Written with 0600 permissions, separate from preferences in config.yaml.

Env override:
  BITRISE_TOKEN takes precedence over the saved token; useful for CI.`,
	}
	c.AddCommand(
		newAuthLoginCmd(),
		newAuthLogoutCmd(),
		newAuthStatusCmd(),
	)
	return c
}

func newAuthLoginCmd() *cobra.Command {
	var withToken bool
	c := &cobra.Command{
		Use:   "login",
		Short: "Save a Bitrise access token",
		Long: `Save a Bitrise access token for future commands to use.

By default the command prompts for the token (input is masked when stdin
is a terminal). Use --with-token to read the token from stdin without a
prompt — the right choice for piping or scripts:

  echo "$BITRISE_TOKEN" | bitrise-cli auth login --with-token

The token is written to $XDG_CONFIG_HOME/bitrise/auth.yaml with 0600
permissions. The token is NOT echoed in any output (use 'auth status' to
verify, 'auth logout' to clear).`,
		Example: `  bitrise-cli auth login                                # interactive prompt
  echo "$BITRISE_TOKEN" | bitrise-cli auth login --with-token`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tok, err := readTokenInput(cmd.InOrStdin(), cmd.ErrOrStderr(), withToken)
			if err != nil {
				return err
			}
			if tok == "" {
				return fmt.Errorf("token is empty")
			}
			if err := auth.Save(auth.Auth{Token: tok}); err != nil {
				return err
			}
			if !quiet {
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Saved access token"); err != nil {
					return err
				}
			}
			return nil
		},
	}
	c.Flags().BoolVar(&withToken, "with-token", false, "read token from stdin without an interactive prompt")
	return c
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the saved access token",
		Long: `Remove the auth.yaml file. Does not affect tokens set via the
BITRISE_TOKEN environment variable or the legacy 'config set token'.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := auth.Clear(); err != nil {
				return err
			}
			if !quiet {
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Cleared saved access token"); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// authStatus is the JSON shape of `bitrise-cli auth status`.
type authStatus struct {
	HasToken  bool   `json:"has_token"`
	TokenType string `json:"token_type,omitempty"`
	Source    string `json:"source"`
	Path      string `json:"path"`
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether an access token is configured and where it came from",
		Long: `Show whether an access token is configured and which source supplied it.

Sources, in precedence order:
  env             BITRISE_TOKEN environment variable
  auth file       auth.yaml (set via 'bitrise-cli auth login')
  legacy config   token field in config.yaml (set via 'config set token')
  none            no token configured`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r := resolvedFromCmd(cmd)
			p, err := auth.Path()
			if err != nil {
				return err
			}
			s := authStatus{
				HasToken: r.Token != "",
				Path:     p,
				Source:   tokenSource(r.Token),
			}
			if r.Token != "" {
				s.TokenType = auth.TokenType(r.Token)
			}
			return output.Render(cmd.OutOrStdout(), resolveFormat(cmd), s, renderAuthStatusHuman)
		},
	}
}

// tokenSource reports which configuration layer supplied the resolved token.
// It re-checks the env var directly because Resolved doesn't carry the source.
func tokenSource(resolvedToken string) string {
	if resolvedToken == "" {
		return "none"
	}
	if os.Getenv(config.EnvToken) != "" {
		return "env (" + config.EnvToken + ")"
	}
	// We can't distinguish auth.yaml from legacy config.yaml from Resolved
	// alone. Re-read auth.yaml; if it has a token, that's the source.
	if a, err := auth.Load(); err == nil && a.Token != "" {
		return "auth file"
	}
	return "legacy config (config.yaml)"
}

func renderAuthStatusHuman(w io.Writer, st authStatus) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	if !st.HasToken {
		ew.F("%s No access token configured.\n\n", s.Failure.Render("✗"))
		ew.Ln("Run 'bitrise-cli auth login' to save one,")
		ew.Ln("or set the BITRISE_TOKEN environment variable.")
		return ew.Err
	}
	lbl := func(label string) string {
		return s.Label.Render(fmt.Sprintf("%-16s", label))
	}
	ew.F("%s %s\n", s.Success.Render("✓"), s.Bold.Render("Access token configured"))
	ew.F("%s%s\n", lbl("Token:"), s.Dim.Render("******** (set)"))
	ew.F("%s%s\n", lbl("Type:"), st.TokenType)
	ew.F("%s%s\n", lbl("Source:"), st.Source)
	ew.F("%s%s\n", lbl("Path:"), s.Dim.Render(st.Path))
	return ew.Err
}

// readTokenInput reads a token via interactive masked prompt or directly
// from stdin. When stdin is not a TTY (piped/redirected) it always reads
// without prompting, regardless of withToken.
func readTokenInput(in io.Reader, stderr io.Writer, withToken bool) (string, error) {
	if !withToken {
		if f, ok := in.(*os.File); ok {
			fd := int(f.Fd()) //nolint:gosec // file descriptors are small ints, no overflow risk
			if term.IsTerminal(fd) {
				if _, err := fmt.Fprint(stderr, "Paste your Bitrise token: "); err != nil {
					return "", err
				}
				b, err := term.ReadPassword(fd)
				if _, perr := fmt.Fprintln(stderr); perr != nil { // newline after no-echo input
					return "", perr
				}
				if err != nil {
					return "", err
				}
				return strings.TrimSpace(string(b)), nil
			}
		}
	}
	// Either --with-token was passed, or stdin isn't a TTY: just read a line.
	s, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(s), nil
}
