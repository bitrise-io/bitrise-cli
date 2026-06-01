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
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
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

// ResolveWorkspaceID returns the RDE workspace ID (== workspace slug),
// preferring --workspace, then BITRISE_WORKSPACE_ID, then the
// default_workspace_slug config key (per the RDE plan, the workspaceId
// is the workspace slug).
func ResolveWorkspaceID(cmd *cobra.Command) (string, error) {
	if v, _ := cmd.Flags().GetString(FlagWorkspace); v != "" {
		return v, nil
	}
	if v := config.FromContext(cmd.Context()).WorkspaceID; v != "" {
		return v, nil
	}
	return "", fmt.Errorf("--workspace is required (or set %s, or run 'bitrise-cli config set %s <slug>')",
		config.EnvWorkspaceID, config.KeyOrgSlug)
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

// ErrNoToken is returned by NewAPIClient when no Bitrise access token has
// been resolved from any layer (env, auth.yaml, or legacy config).
var ErrNoToken = errors.New("no Bitrise access token configured (run 'bitrise-cli auth login' or set BITRISE_TOKEN)")

// NewAPIClient builds a *bitriseapi.Client from the Resolved settings on
// cmd.Context(). Returns ErrNoToken if no token is set anywhere.
func NewAPIClient(cmd *cobra.Command) (*bitriseapi.Client, error) {
	r := config.FromContext(cmd.Context())
	if r.Token == "" {
		return nil, ErrNoToken
	}
	return bitriseapi.New(r.APIBaseURL, r.Token), nil
}

// NewRDEClient builds an *rdeapi.Client for the Remote Dev Environments API
// from the Resolved settings on cmd.Context(). Returns ErrNoToken if no
// token is set anywhere. The RDE service uses Bearer auth (vs the legacy
// "token <PAT>" header on the main bitriseapi client).
func NewRDEClient(cmd *cobra.Command) (*rdeapi.Client, error) {
	r := config.FromContext(cmd.Context())
	if r.Token == "" {
		return nil, ErrNoToken
	}
	return rdeapi.New(r.RDEAPIBaseURL, r.Token), nil
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

// ReadSecretInput reads a secret (token, password) from in. When fromStdin
// is true, or when in is not a terminal, it reads a single line directly.
// Otherwise it prints prompt to stderr, reads a masked line, and writes a
// trailing newline. The trimmed value is returned with surrounding
// whitespace removed.
func ReadSecretInput(in io.Reader, stderr io.Writer, prompt string, fromStdin bool) (string, error) {
	if !fromStdin {
		if f, ok := in.(*os.File); ok {
			fd := int(f.Fd()) //nolint:gosec // file descriptors are small ints, no overflow risk
			if term.IsTerminal(fd) {
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
		}
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
