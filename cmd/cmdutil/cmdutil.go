// Package cmdutil holds helpers shared across the cmd sub-packages.
package cmdutil

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
	"github.com/bitrise-io/bitrise-cli/internal/cache"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/oauth"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/resolve"
)

const (
	FlagOutput    = "output"
	FlagApp       = "app"
	FlagWorkspace = "workspace"
	FlagQuiet     = "quiet"
	FlagWeb       = "web"
	FlagTheme     = "theme"
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

// ResolveWebBaseURL returns the resolved web base URL (https://app.bitrise.io
// in normal use, overridable via BITRISE_WEB_BASE_URL or the web_base_url
// config key). Used by user-creation and email/password login commands that
// talk to the website's JSON endpoints.
func ResolveWebBaseURL(cmd *cobra.Command) string {
	if u := config.FromContext(cmd.Context()).WebBaseURL; u != "" {
		return u
	}
	return config.DefaultWebBaseURL
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

// ResolveWorkspaceID returns the workspace ID, preferring --workspace, then
// BITRISE_WORKSPACE_ID, then the default_workspace_id config key. When none is
// set and the account has exactly one workspace, that workspace is used
// automatically (one GET /organizations call); 0 or 2+ workspaces produce a
// friendly error. (On the Bitrise API this identifier is a slug; the CLI never
// exposes that term.)
func ResolveWorkspaceID(cmd *cobra.Command) (string, error) {
	if v, _ := cmd.Flags().GetString(FlagWorkspace); v != "" {
		return v, nil
	}
	if v := config.FromContext(cmd.Context()).WorkspaceID; v != "" {
		return v, nil
	}
	// Nothing configured — fall back to the account's sole workspace.
	client, err := NewAPIClient(cmd)
	if err != nil {
		return "", err
	}
	ws, err := NewResolver(cmd, client).DefaultWorkspace(cmd.Context())
	if err != nil {
		return "", err
	}
	// Announce the auto-pick and nudge the user to persist it, but only for
	// human output — in JSON (scripting/CI) these hints are just noise. The
	// breadcrumb already goes to stderr, so it never touches the JSON on stdout.
	if !IsQuiet(cmd) && ResolveFormat(cmd) == output.Human {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Using your only workspace: %s\n", workspaceLabel(ws))
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Set it permanently to skip this lookup: bitrise-cli config set %s %s\n", config.KeyDefaultWorkspaceID, ws.Slug)
	}
	return ws.Slug, nil
}

// workspaceLabel renders a workspace for a human breadcrumb as "name (id)",
// falling back to the bare ID when the API omitted a name.
func workspaceLabel(ws bitriseapi.Organization) string {
	if ws.Name != "" {
		return fmt.Sprintf("%s (%s)", ws.Name, ws.Slug)
	}
	return ws.Slug
}

// ResolveAppSlugArg returns the positional APP_ID argument, falling back to Resolved.
func ResolveAppSlugArg(cmd *cobra.Command, args []string) (string, error) {
	if len(args) >= 1 && args[0] != "" {
		return args[0], nil
	}
	if v := config.FromContext(cmd.Context()).AppSlug; v != "" {
		return v, nil
	}
	return "", AppSlugRequiredErr("APP_ID positional argument")
}

// AppSlugRequiredErr returns the standard missing-app-slug error.
func AppSlugRequiredErr(via string) error {
	return fmt.Errorf("%s is required (or set %s, or run 'bitrise-cli config set %s <id>')",
		via, config.EnvAppSlug, config.KeyAppID)
}

// NewResolver returns a Resolver wired to client and a fresh in-memory cache.
func NewResolver(cmd *cobra.Command, client *bitriseapi.Client) *resolve.Resolver {
	return resolve.New(client, cache.New())
}

// ResolveAndLookupAppSlug reads the app slug from --app / env / config (same
// precedence as ResolveAppSlug), then resolves a display name to an app slug
// via a targeted GET /apps?title=<value> query if the value doesn't match any
// slug directly.
func ResolveAndLookupAppSlug(cmd *cobra.Command, client *bitriseapi.Client) (string, error) {
	raw, err := ResolveAppSlug(cmd)
	if err != nil {
		return "", err
	}
	return NewResolver(cmd, client).AppSlug(cmd.Context(), raw)
}

// ErrNoToken is returned by NewAPIClient when no Bitrise access token has
// been resolved from any layer (env, auth.yaml, or legacy config).
var ErrNoToken = errors.New("no Bitrise access token configured (run 'bitrise-cli auth login' or set BITRISE_TOKEN)")

// liveToken resolves the access token to use for an API call, refreshing an
// OAuth-managed token if it has expired. It is the single token-resolution
// path for both API clients, so refresh happens for every API-bound command
// but never in Resolve or persistentPreRunE (which also run for version,
// config list, etc.).
//
// BITRISE_TOKEN, when set, is used verbatim and never refreshed — that's the
// CI path. Otherwise the resolved token (from auth.yaml) is handed to the
// OAuth refresh ladder, which is a no-op for a manually pasted/email token.
func liveToken(cmd *cobra.Command) (string, error) {
	if t := os.Getenv(config.EnvToken); t != "" {
		return t, nil
	}
	r := config.FromContext(cmd.Context())
	if r.Token == "" {
		return "", ErrNoToken
	}
	return oauth.NewConfig(r.OAuthIssuer, r.OIDCTokenEndpoint, r.OAuthClientID).EnsureFreshPAT(cmd.Context(), r.Token)
}

// NewAPIClient builds a *bitriseapi.Client from the Resolved settings on
// cmd.Context(). Returns ErrNoToken if no token is set anywhere.
func NewAPIClient(cmd *cobra.Command) (*bitriseapi.Client, error) {
	tok, err := liveToken(cmd)
	if err != nil {
		return nil, err
	}
	r := config.FromContext(cmd.Context())
	return bitriseapi.New(r.APIBaseURL, tok), nil
}

// NewRDEClient builds an *rdeapi.Client for the Remote Dev Environments API
// from the Resolved settings on cmd.Context(). Returns ErrNoToken if no
// token is set anywhere. The RDE service uses Bearer auth (vs the legacy
// "token <PAT>" header on the main bitriseapi client).
func NewRDEClient(cmd *cobra.Command) (*rdeapi.Client, error) {
	tok, err := liveToken(cmd)
	if err != nil {
		return nil, err
	}
	r := config.FromContext(cmd.Context())
	return rdeapi.New(r.RDEAPIBaseURL, tok), nil
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

// SilenceRootErrors prevents cobra from printing a returned error to stderr by
// setting SilenceErrors on both the command and its root. Use this when the
// command has already printed its own error summary and the automatic
// "Error: ..." line would be redundant.
func SilenceRootErrors(cmd *cobra.Command) {
	cmd.SilenceErrors = true
	if root := cmd.Root(); root != nil {
		root.SilenceErrors = true
	}
}

// terminalFd returns the stream's file descriptor and reports whether it is an
// interactive terminal (an *os.File backed by a TTY). It accepts either a
// reader or a writer — it only needs the concrete *os.File — and is the single
// place that inspects it; IsTerminal, IsTerminalWriter, and ReadSecretInput all
// build on it. fd is only meaningful when isTerminal is true.
func terminalFd(stream any) (fd int, isTerminal bool) {
	f, ok := stream.(*os.File)
	if !ok {
		return 0, false
	}
	fd = int(f.Fd()) //nolint:gosec // file descriptors are small ints, no overflow risk
	return fd, term.IsTerminal(fd)
}

// IsTerminal reports whether r is an interactive terminal — an *os.File backed
// by a TTY. Pipes, buffers, and test readers are never terminals, so callers
// can use it to pick an interactive default (e.g. a browser flow) while keeping
// non-interactive stdin (CI, pipes) working.
func IsTerminal(r io.Reader) bool {
	_, ok := terminalFd(r)
	return ok
}

// IsTerminalWriter reports whether w is an interactive terminal. Used to gate
// stderr-only chrome (e.g. the update notice) so it never lands in pipes or
// redirected logs.
func IsTerminalWriter(w io.Writer) bool {
	_, ok := terminalFd(w)
	return ok
}

// ReadSecretInput reads a secret (token, password) from in. When fromStdin
// is true, or when in is not a terminal, it reads a single line directly.
// Otherwise it prints prompt to stderr, reads a masked line, and writes a
// trailing newline. The trimmed value is returned with surrounding
// whitespace removed.
func ReadSecretInput(in io.Reader, stderr io.Writer, prompt string, fromStdin bool) (string, error) {
	if fd, ok := terminalFd(in); ok && !fromStdin {
		if _, err := fmt.Fprint(stderr, prompt); err != nil {
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
	s, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

// RequireArgs returns an Args validator that names exactly which positional
// argument(s) are missing, instead of cobra's generic "accepts N arg(s), received M".
func RequireArgs(names ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) >= len(names) {
			return nil
		}
		missing := names[len(args):]
		var msg string
		if len(missing) == 1 {
			msg = fmt.Sprintf("missing argument: %s", missing[0])
		} else {
			msg = fmt.Sprintf("missing arguments: %s", strings.Join(missing, " "))
		}
		return fmt.Errorf("%s\nRun '%s --help' for usage", msg, cmd.CommandPath())
	}
}

// DelegateToList forwards a bare parent invocation to its "list" subcommand,
// propagating the parent's context so resolved config is available.
func DelegateToList(cmd *cobra.Command, args []string) error {
	for _, sub := range cmd.Commands() {
		if sub.Name() == "list" {
			sub.SetContext(cmd.Context())
			return sub.RunE(sub, args)
		}
	}
	return cmd.Help()
}
