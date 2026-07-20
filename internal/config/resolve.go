package config

import (
	"context"
	"os"

	"github.com/bitrise-io/bitrise-cli/internal/auth"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

// Environment variables that override config-file values.
const (
	EnvToken         = "BITRISE_TOKEN"
	EnvAppSlug       = "BITRISE_APP_ID"
	EnvAppSlugLegacy = "BITRISE_APP_SLUG" // pre-rename name, still accepted; EnvAppSlug wins
	EnvWorkspaceID   = "BITRISE_WORKSPACE_ID"
	// EnvAgentID identifies the automation agent driving the CLI. When set,
	// `rde session create` labels new sessions with agent=<id> and
	// `rde session list --mine` filters on that label.
	EnvAgentID       = "BITRISE_AGENT_ID"
	EnvOutput        = "BITRISE_OUTPUT"
	EnvAPIBaseURL    = "BITRISE_API_BASE_URL"
	EnvRDEAPIBaseURL = "BITRISE_RDE_API_BASE_URL"
	EnvWebBaseURL    = "BITRISE_WEB_BASE_URL"
	EnvTheme         = "BITRISE_CLI_THEME"
	// EnvOAuthIssuer overrides the WorkOS AuthKit issuer (full URL) the OAuth
	// login flow authorizes against; EnvOIDCTokenEndpoint overrides the
	// monolith JWT→PAT exchange endpoint (full URL); EnvOAuthClientID overrides
	// the CLI's CIMD client_id URL. These point the flow at a non-prod
	// environment — the client_id differs per environment because its metadata
	// document is served from each environment's own host. None is secret.
	EnvOAuthIssuer       = "BITRISE_OAUTH_ISSUER"
	EnvOIDCTokenEndpoint = "BITRISE_OIDC_TOKEN_ENDPOINT" //nolint:gosec // G101: env var name, not a credential
	EnvOAuthClientID     = "BITRISE_OAUTH_CLIENT_ID"
)

// DefaultAPIBaseURL is the production Bitrise API base URL.
const DefaultAPIBaseURL = "https://api.bitrise.io/v0.1"

// DefaultRDEAPIBaseURL is the production Bitrise Remote Dev Environments
// API base URL. Endpoints under this host follow the swagger published at
// https://api.bitrise.io/rde/api-docs/swagger.json.
const DefaultRDEAPIBaseURL = "https://api.bitrise.io/rde"

// DefaultWebBaseURL is the production Bitrise web app base URL.
// Used by `user create` and `auth login --email` to drive the website's
// signup and sign-in JSON endpoints.
const DefaultWebBaseURL = "https://app.bitrise.io"

// DefaultOIDCTokenEndpoint is the production monolith endpoint that exchanges
// a WorkOS-issued JWT for a Bitrise PAT (RFC 8693 token exchange), used by the
// `auth login --oauth` flow and its background token refresh.
const DefaultOIDCTokenEndpoint = "https://app.bitrise.io/oidc/token" //nolint:gosec // G101: public endpoint URL, not a credential

// DefaultOAuthIssuer is the WorkOS AuthKit domain (issuer) the OAuth login
// flow authorizes against; it hosts /oauth2/authorize and /oauth2/token. It's
// the production WorkOS environment's custom domain (shared with the MCP
// server, whose EXTERNAL_OAUTH_ISSUER must match) — not a secret. Override with
// BITRISE_OAUTH_ISSUER to target a different (e.g. staging) environment.
const DefaultOAuthIssuer = "https://oauth.bitrise.io"

// DefaultOAuthClientID is the production CLI OAuth client_id: the URL of the
// Client ID Metadata Document (CIMD) the monolith serves at app.bitrise.io.
// WorkOS fetches it at authorize time — the URL *is* the id, and it is not a
// secret. It is host-specific (the document is served from each environment's
// own root URL), so non-prod environments override it via
// BITRISE_OAUTH_CLIENT_ID. The prod monolith derives the same value from its
// app root URL, keeping the two in sync.
const DefaultOAuthClientID = "https://app.bitrise.io/.well-known/oauth-client/cli"

// Resolved is the merged settings the cmd layer reads on every invocation.
//
// Layering per the CLI patterns guide, highest to lowest precedence:
//  1. CLI flag (only the persistent --output flag is folded in here;
//     per-command flags like --app are layered in the command handlers)
//  2. Environment variables
//  3. Per-directory config (.bitrise-cli.yml in CWD or ancestors)
//  4. Global config file (~/.config/bitrise/config.yaml) — for non-secret keys
//  5. auth.yaml (~/.config/bitrise/auth.yaml) — for the token only
//  6. Built-in defaults
//
// Token resolution: env > auth.yaml.
type Resolved struct {
	Output        output.Format
	AppSlug       string
	OrgSlug       string
	WorkspaceID   string
	Token         string
	APIBaseURL    string
	RDEAPIBaseURL string
	WebBaseURL    string
	// OAuthIssuer, OIDCTokenEndpoint, and OAuthClientID configure the
	// `auth login --oauth` flow and its background token refresh. They default
	// to production and are overridable per environment via the matching
	// BITRISE_OAUTH_* / BITRISE_OIDC_* env vars.
	OAuthIssuer       string
	OIDCTokenEndpoint string
	OAuthClientID     string
	Theme             style.Theme
}

// Resolve merges global config, per-directory config, the auth file, and
// environment variables with the persistent --output / --theme flag values.
// flagOutput / flagTheme may be empty when unset. dirCfg / authData are zero
// values when their respective files were not found.
func Resolve(globalCfg, dirCfg Config, authData auth.Auth, flagOutput, flagTheme string) (Resolved, error) {
	var r Resolved

	rawOutput := flagOutput
	if rawOutput == "" {
		rawOutput = firstNonEmpty(os.Getenv(EnvOutput), dirCfg.Output, globalCfg.Output)
	}
	f, err := output.ParseFormat(rawOutput)
	if err != nil {
		return Resolved{}, err
	}
	r.Output = f

	rawTheme := flagTheme
	if rawTheme == "" {
		rawTheme = firstNonEmpty(os.Getenv(EnvTheme), dirCfg.Theme, globalCfg.Theme)
	}
	t, err := style.ParseTheme(rawTheme)
	if err != nil {
		return Resolved{}, err
	}
	r.Theme = t

	r.AppSlug = firstNonEmpty(os.Getenv(EnvAppSlug), os.Getenv(EnvAppSlugLegacy), dirCfg.AppID, globalCfg.AppID)
	r.OrgSlug = firstNonEmpty(dirCfg.DefaultWorkspaceID, globalCfg.DefaultWorkspaceID)
	// WorkspaceID resolution: BITRISE_WORKSPACE_ID env, then fall back to the
	// existing default_workspace_id — the RDE workspaceId is the same
	// workspace identifier we already store (a slug on the wire).
	r.WorkspaceID = firstNonEmpty(os.Getenv(EnvWorkspaceID), r.OrgSlug)
	r.APIBaseURL = firstNonEmpty(os.Getenv(EnvAPIBaseURL), dirCfg.APIBaseURL, globalCfg.APIBaseURL, DefaultAPIBaseURL)
	r.RDEAPIBaseURL = firstNonEmpty(os.Getenv(EnvRDEAPIBaseURL), dirCfg.RDEAPIBaseURL, globalCfg.RDEAPIBaseURL, DefaultRDEAPIBaseURL)
	r.WebBaseURL = firstNonEmpty(os.Getenv(EnvWebBaseURL), dirCfg.WebBaseURL, globalCfg.WebBaseURL, DefaultWebBaseURL)
	r.OAuthIssuer = firstNonEmpty(os.Getenv(EnvOAuthIssuer), DefaultOAuthIssuer)
	r.OIDCTokenEndpoint = firstNonEmpty(os.Getenv(EnvOIDCTokenEndpoint), DefaultOIDCTokenEndpoint)
	r.OAuthClientID = firstNonEmpty(os.Getenv(EnvOAuthClientID), DefaultOAuthClientID)
	r.Token = firstNonEmpty(os.Getenv(EnvToken), authData.Token)

	return r, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

type ctxKey struct{}

// WithResolved stores r on ctx so command handlers can read it.
func WithResolved(ctx context.Context, r Resolved) context.Context {
	return context.WithValue(ctx, ctxKey{}, r)
}

// FromContext retrieves Resolved from ctx, or a zero value if absent.
// The zero value's Output ("") will fail at format dispatch — callers
// should never see it in practice because root's PersistentPreRunE always
// installs a Resolved before any subcommand runs.
func FromContext(ctx context.Context) Resolved {
	if r, ok := ctx.Value(ctxKey{}).(Resolved); ok {
		return r
	}
	return Resolved{}
}
