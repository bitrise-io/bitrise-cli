// Package oauth implements the browser-based OAuth login flow for the Bitrise
// CLI and the transparent token refresh that keeps it working.
//
// The CLI is a public OAuth client (no client secret — PKCE replaces it). The
// login dance is the standard authorization-code + PKCE flow against Bitrise's
// WorkOS AuthKit environment, with a loopback redirect (RFC 8252):
//
//  1. authorize at <issuer>/oauth2/authorize, user signs in via the browser;
//  2. exchange the returned code for a JWT at <issuer>/oauth2/token;
//  3. exchange that JWT for a Bitrise PAT at the monolith's OIDC token
//     endpoint (RFC 8693 token exchange — the same call the MCP server makes).
//
// The PAT is the working credential every command uses, stored on disk exactly
// like a pasted token. The JWT, refresh token, and expiries are stored
// alongside it so EnsureFreshPAT can mint a new PAT without a browser when the
// old one expires. None of the identity inputs are secret, so they ship in the
// open-source binary.
//
// This package depends only on internal/auth and the standard library; it must
// not import internal/config or cmd/* (the cmd layer bridges config.Resolved
// into a Config).
package oauth

import (
	"net/http"
	"strings"
	"time"
)

// DefaultResource is the CLI's audience / resource indicator. Sent on the
// authorize request so WorkOS pins it into the JWT `aud`. It must be registered
// as a Resource Indicator in the WorkOS dashboard for WorkOS to honour it; the
// monolith already accepts any *.bitrise.io audience at /oidc/token. Mirrors
// the MCP server's https://mcp.bitrise.io.
//
// Unlike the client_id (a per-environment CIMD URL, resolved by the config
// layer and overridable via BITRISE_OAUTH_CLIENT_ID), the resource is a stable
// identifier shared across environments, so it stays a compile-time constant.
const DefaultResource = "https://cli.bitrise.io"

// defaultTimeout bounds each token HTTP call. defaultPATLifetime is the
// fallback PAT lifetime when the exchange response omits expires_in (the
// monolith's PAT_EXPIRY is 1h). refreshSkew re-mints slightly before the real
// expiry so a token never goes stale mid-request.
const (
	defaultTimeout     = 30 * time.Second
	defaultPATLifetime = time.Hour
	refreshSkew        = 60 * time.Second
)

// Config carries the external inputs for the OAuth flow. The cmd layer builds
// one with NewConfig (resolved issuer/endpoint/client_id + package-default
// resource); tests construct it directly with their own httptest URLs.
type Config struct {
	// Issuer is the WorkOS AuthKit domain hosting /oauth2/authorize and
	// /oauth2/token. May be empty when no default is compiled in and none is
	// set via BITRISE_OAUTH_ISSUER — Login reports a clear error in that case.
	Issuer string
	// OIDCTokenEndpoint is the monolith endpoint that exchanges a JWT for a PAT.
	OIDCTokenEndpoint string
	// ClientID is the CIMD URL identifying this client.
	ClientID string
	// Resource is the audience/resource indicator pinned into the JWT.
	Resource string
	// HTTPClient, when set, overrides the default client (used by tests).
	HTTPClient *http.Client
}

// NewConfig returns a Config for the OAuth flow from the supplied (resolved)
// issuer, OIDC token endpoint, and client_id, with the package-default
// resource. client_id is passed in rather than a constant because it is a
// per-environment CIMD URL the config layer resolves (config.DefaultOAuthClientID,
// overridable via BITRISE_OAUTH_CLIENT_ID).
func NewConfig(issuer, oidcTokenEndpoint, clientID string) Config {
	return Config{
		Issuer:            issuer,
		OIDCTokenEndpoint: oidcTokenEndpoint,
		ClientID:          clientID,
		Resource:          DefaultResource,
	}
}

func (c Config) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: defaultTimeout}
}

func (c Config) authorizeEndpoint() string {
	return strings.TrimRight(c.Issuer, "/") + "/oauth2/authorize"
}

func (c Config) tokenEndpoint() string {
	return strings.TrimRight(c.Issuer, "/") + "/oauth2/token"
}
