package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/auth"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/oauth"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internaluser "github.com/bitrise-io/bitrise-cli/internal/user"
	"github.com/bitrise-io/bitrise-cli/internal/webclient"
)

// newAuthCmd returns the `bitrise-cli auth` parent command and its subcommands.
//
// The auth surface is the way to set Bitrise credentials: `auth login` writes
// the token to a separate auth.yaml so credentials live apart from preferences
// in config.yaml. A token can also be supplied via the BITRISE_TOKEN env var,
// which takes precedence over the saved file.
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
	var (
		withToken     bool
		emailLogin    string
		passwordStdin bool
		oauthLogin    bool
		webLogin      bool
	)
	c := &cobra.Command{
		Use:   "login",
		Short: "Save a Bitrise access token",
		Long: `Save a Bitrise access token for future commands to use.

There are three modes:

  1. Token paste (default).
     Prompts for a Personal Access Token (or pipes one in with --with-token).
     The token is masked when stdin is a terminal:

         bitrise-cli auth login
         echo "$BITRISE_TOKEN" | bitrise-cli auth login --with-token

  2. Email and password (--email).
     Signs in to app.bitrise.io with your account credentials, then asks the
     server to mint a fresh Personal Access Token and stores it. The cookie
     session used to mint the token is dropped immediately. Your account
     must have its email verified — run 'bitrise-cli user create' first if
     you don't yet have an account:

         bitrise-cli auth login --email alice@example.com
         printf '%s' "$PW" | bitrise-cli auth login --email alice@example.com --password-stdin

  3. Browser sign-in (--oauth).
     Opens your browser to sign in to Bitrise, then exchanges the result for a
     Personal Access Token and stores it. The CLI keeps the credential fresh in
     the background, so you rarely need to sign in again:

         bitrise-cli auth login --oauth

     This requires the browser to run on the same machine as the CLI (the sign-in
     is handed back over a loopback address). Signing in on a remote/headless
     host over SSH is not yet supported — paste a token with --with-token there.

Either way the resulting token is written to
$XDG_CONFIG_HOME/bitrise/auth.yaml with 0600 permissions. The token is NOT
echoed in any output (use 'auth status' to verify, 'auth logout' to clear).`,
		Example: `  bitrise-cli auth login                                       # interactive token prompt
  echo "$BITRISE_TOKEN" | bitrise-cli auth login --with-token
  bitrise-cli auth login --oauth                               # sign in via the browser
  bitrise-cli auth login --email alice@example.com             # interactive password prompt
  printf '%s' "$PW" | bitrise-cli auth login --email alice@example.com --password-stdin`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if oauthLogin || webLogin {
				return runOAuthLogin(cmd)
			}
			if emailLogin != "" {
				return runEmailLogin(cmd, emailLogin, passwordStdin)
			}
			if passwordStdin {
				return fmt.Errorf("--password-stdin requires --email (token login reads the token, not a password)")
			}
			return runTokenLogin(cmd, withToken)
		},
	}
	c.Flags().BoolVar(&withToken, "with-token", false, "read token from stdin without an interactive prompt")
	c.Flags().StringVar(&emailLogin, "email", "", "sign in by email/password and mint a Personal Access Token")
	c.Flags().BoolVar(&passwordStdin, "password-stdin", false, "with --email, read the password from stdin without prompting")
	c.Flags().BoolVar(&oauthLogin, "oauth", false, "sign in via the browser (OAuth) and store a managed, auto-refreshing token")
	// --web is a hidden alias for --oauth ("open in the browser").
	c.Flags().BoolVar(&webLogin, cmdutil.FlagWeb, false, "alias for --oauth")
	_ = c.Flags().MarkHidden(cmdutil.FlagWeb)
	// The three login modes are mutually exclusive. --oauth and --web are
	// aliases, so they're not exclusive with each other.
	for _, mode := range []string{"oauth", cmdutil.FlagWeb} {
		c.MarkFlagsMutuallyExclusive(mode, "with-token")
		c.MarkFlagsMutuallyExclusive(mode, "email")
		c.MarkFlagsMutuallyExclusive(mode, "password-stdin")
	}
	c.MarkFlagsMutuallyExclusive("with-token", "email")
	c.MarkFlagsMutuallyExclusive("with-token", "password-stdin")
	return c
}

func runTokenLogin(cmd *cobra.Command, withToken bool) error {
	tok, err := cmdutil.ReadSecretInput(cmd.InOrStdin(), cmd.ErrOrStderr(), "Paste your Bitrise token: ", withToken)
	if err != nil {
		return err
	}
	if tok == "" {
		return fmt.Errorf("token is empty")
	}
	if err := auth.Save(auth.Auth{Token: tok}); err != nil {
		return err
	}
	return confirmLoginSaved(cmd)
}

func runEmailLogin(cmd *cobra.Command, email string, passwordStdin bool) error {
	pw, err := cmdutil.ReadSecretInput(cmd.InOrStdin(), cmd.ErrOrStderr(), "Password: ", passwordStdin)
	if err != nil {
		return err
	}
	if pw == "" {
		return fmt.Errorf("password is empty")
	}
	wc, err := webclient.New(cmdutil.ResolveWebBaseURL(cmd))
	if err != nil {
		return err
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "unknown-host"
	}
	svc := internaluser.NewService(wc)
	tok, err := svc.Login(cmd.Context(), internaluser.LoginInput{Login: email, Password: pw}, fmt.Sprintf("bitrise-cli (%s)", host))
	if err != nil {
		if internaluser.IsUnconfirmedEmailErr(err) {
			return fmt.Errorf("this account hasn't verified its email yet — click the link in the confirmation email, then re-run")
		}
		return err
	}
	if err := auth.Save(auth.Auth{Token: tok}); err != nil {
		return err
	}
	return confirmLoginSaved(cmd)
}

// runOAuthLogin drives the browser-based OAuth flow: it runs the authorization
// dance via internal/oauth, exchanges the result for a Personal Access Token,
// and persists the PAT plus the refresh material that keeps it fresh.
func runOAuthLogin(cmd *cobra.Command) error {
	r := resolvedFromCmd(cmd)
	a, err := oauth.NewConfig(r.OAuthIssuer, r.OIDCTokenEndpoint, r.OAuthClientID).
		Login(cmd.Context(), cmdutil.OpenBrowser, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	if err := auth.Save(a); err != nil {
		return err
	}
	return confirmLoginSaved(cmd)
}

// confirmLoginSaved reports a successful login on stderr (unless --quiet) and,
// when BITRISE_TOKEN is set, warns that it shadows the token just saved.
// BITRISE_TOKEN takes precedence over auth.yaml (see config resolution), so
// without this notice the login would silently have no effect on later commands
// — a common cause of a confusing 401. The warning is shown even under --quiet,
// since it means the login didn't take effect.
func confirmLoginSaved(cmd *cobra.Command) error {
	ew := cmdutil.NewErrWriter(cmd.ErrOrStderr())
	if !quiet {
		ew.Ln("Saved access token")
	}
	if os.Getenv(config.EnvToken) != "" {
		s := style.New(cmd.ErrOrStderr())
		ew.F("%s %s is set and takes precedence over the token just saved.\n", s.Warn.Render("Warning:"), config.EnvToken)
		ew.F("         Commands will use it, not this login — run 'unset %s' to use the saved token.\n", config.EnvToken)
	}
	return ew.Err
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
	// TokenExpiry is set (RFC 3339) only for OAuth-managed tokens, whose
	// expiry the CLI tracks for background refresh. No token material is
	// ever included.
	TokenExpiry string `json:"token_expiry,omitempty"`
	Path        string `json:"path"`
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether an access token is configured and where it came from",
		Long: `Show whether an access token is configured and which source supplied it.

Sources, in precedence order:
  env        BITRISE_TOKEN environment variable
  auth file  auth.yaml, written by 'bitrise-cli auth login' (OAuth or a
             pasted/email token — a new login overwrites the previous one).
             OAuth logins are shown as "oauth (auth file)" and refreshed
             automatically.
  none       no token configured`,
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
				// When the token comes from the auth file (not env), surface
				// OAuth-managed details: a clearer source label and the PAT
				// expiry the CLI refreshes against. Token material is omitted.
				if os.Getenv(config.EnvToken) == "" {
					if a, err := auth.Load(); err == nil && a.IsOAuthManaged() {
						s.Source = "oauth (auth file)"
						if !a.TokenExpiry.IsZero() {
							s.TokenExpiry = a.TokenExpiry.Format(time.RFC3339)
						}
					}
				}
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
	// A non-env token can only have come from auth.yaml — it's the sole
	// persisted source (config.yaml holds no token).
	return "auth file"
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
	if st.TokenExpiry != "" {
		ew.F("%s%s\n", lbl("Expires:"), st.TokenExpiry)
	}
	ew.F("%s%s\n", lbl("Path:"), s.Dim.Render(st.Path))
	return ew.Err
}
