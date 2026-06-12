package config

import (
	"testing"

	"github.com/bitrise-io/bitrise-cli/internal/auth"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

// clearEnv unsets every env var Resolve consults so a test starts from a
// clean slate regardless of the developer's local shell.
func clearEnv(t *testing.T) {
	t.Helper()
	t.Setenv(EnvOutput, "")
	t.Setenv(EnvAppSlug, "")
	t.Setenv(EnvAppSlugLegacy, "")
	t.Setenv(EnvWorkspaceID, "")
	t.Setenv(EnvToken, "")
	t.Setenv(EnvAPIBaseURL, "")
	t.Setenv(EnvRDEAPIBaseURL, "")
	t.Setenv(EnvWebBaseURL, "")
	t.Setenv(EnvTheme, "")
	t.Setenv(EnvOAuthIssuer, "")
	t.Setenv(EnvOIDCTokenEndpoint, "")
	t.Setenv(EnvOAuthClientID, "")
}

func TestResolve_DefaultsWhenNothingSet(t *testing.T) {
	clearEnv(t)
	r, err := Resolve(Config{}, Config{}, auth.Auth{}, "", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.Output != output.Human {
		t.Errorf("Output = %q, want human", r.Output)
	}
	if r.APIBaseURL != DefaultAPIBaseURL {
		t.Errorf("APIBaseURL = %q, want %q", r.APIBaseURL, DefaultAPIBaseURL)
	}
	if r.RDEAPIBaseURL != DefaultRDEAPIBaseURL {
		t.Errorf("RDEAPIBaseURL = %q, want %q", r.RDEAPIBaseURL, DefaultRDEAPIBaseURL)
	}
	if r.Token != "" || r.AppSlug != "" {
		t.Errorf("expected empty Token/AppSlug, got %+v", r)
	}
}

func TestResolve_OAuthDefaultsAndClientIDOverride(t *testing.T) {
	clearEnv(t)

	// With nothing set, the production OAuth values compile in.
	r, err := Resolve(Config{}, Config{}, auth.Auth{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.OAuthIssuer != DefaultOAuthIssuer {
		t.Errorf("OAuthIssuer = %q, want %q", r.OAuthIssuer, DefaultOAuthIssuer)
	}
	if r.OIDCTokenEndpoint != DefaultOIDCTokenEndpoint {
		t.Errorf("OIDCTokenEndpoint = %q, want %q", r.OIDCTokenEndpoint, DefaultOIDCTokenEndpoint)
	}
	if r.OAuthClientID != DefaultOAuthClientID {
		t.Errorf("OAuthClientID = %q, want %q", r.OAuthClientID, DefaultOAuthClientID)
	}

	// BITRISE_OAUTH_CLIENT_ID overrides the default (e.g. pointing at staging,
	// whose CIMD doc is served from its own host).
	const staging = "https://app-staging.example/.well-known/oauth-client/cli"
	t.Setenv(EnvOAuthClientID, staging)
	r, err = Resolve(Config{}, Config{}, auth.Auth{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.OAuthClientID != staging {
		t.Errorf("OAuthClientID = %q, want %q (env override)", r.OAuthClientID, staging)
	}
}

func TestResolve_WorkspaceIDFallsBackToOrgSlug(t *testing.T) {
	clearEnv(t)

	// With no env var, WorkspaceID falls back to default_workspace_id.
	r, err := Resolve(Config{DefaultWorkspaceID: "acme"}, Config{}, auth.Auth{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.WorkspaceID != "acme" {
		t.Errorf("WorkspaceID = %q, want acme (fallback to org slug)", r.WorkspaceID)
	}

	// BITRISE_WORKSPACE_ID wins over the org slug.
	t.Setenv(EnvWorkspaceID, "ws-env")
	r, err = Resolve(Config{DefaultWorkspaceID: "acme"}, Config{}, auth.Auth{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.WorkspaceID != "ws-env" {
		t.Errorf("WorkspaceID = %q, want ws-env (env wins)", r.WorkspaceID)
	}
}

func TestResolve_RDEAPIBaseURLPrecedence(t *testing.T) {
	clearEnv(t)

	r, err := Resolve(Config{RDEAPIBaseURL: "https://global.rde"}, Config{RDEAPIBaseURL: "https://dir.rde"}, auth.Auth{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.RDEAPIBaseURL != "https://dir.rde" {
		t.Errorf("RDEAPIBaseURL = %q, want dir value", r.RDEAPIBaseURL)
	}

	t.Setenv(EnvRDEAPIBaseURL, "https://env.rde")
	r, err = Resolve(Config{RDEAPIBaseURL: "https://global.rde"}, Config{RDEAPIBaseURL: "https://dir.rde"}, auth.Auth{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.RDEAPIBaseURL != "https://env.rde" {
		t.Errorf("RDEAPIBaseURL = %q, want env value", r.RDEAPIBaseURL)
	}
}

func TestResolve_OutputPrecedence(t *testing.T) {
	clearEnv(t)
	global := Config{Output: "human"}
	dir := Config{Output: "json"}
	t.Setenv(EnvOutput, "human")

	// Flag wins over env, dir, and global.
	r, err := Resolve(global, dir, auth.Auth{}, "json", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Output != output.JSON {
		t.Errorf("flag-wins: Output = %q, want json", r.Output)
	}

	// No flag → env beats dir + global.
	r, err = Resolve(global, dir, auth.Auth{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Output != output.Human {
		t.Errorf("env-wins: Output = %q, want human", r.Output)
	}

	// Clear env → dir beats global.
	t.Setenv(EnvOutput, "")
	r, err = Resolve(global, dir, auth.Auth{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Output != output.JSON {
		t.Errorf("dir-wins: Output = %q, want json", r.Output)
	}

	// Clear dir → global wins.
	r, err = Resolve(global, Config{}, auth.Auth{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Output != output.Human {
		t.Errorf("global-wins: Output = %q, want human", r.Output)
	}
}

func TestResolve_AppSlugPrecedence(t *testing.T) {
	clearEnv(t)

	// global only
	r, _ := Resolve(Config{AppID: "global"}, Config{}, auth.Auth{}, "", "")
	if r.AppSlug != "global" {
		t.Errorf("global-only: %q", r.AppSlug)
	}

	// dir overrides global
	r, _ = Resolve(Config{AppID: "global"}, Config{AppID: "dir"}, auth.Auth{}, "", "")
	if r.AppSlug != "dir" {
		t.Errorf("dir-over-global: %q", r.AppSlug)
	}

	// env overrides everything
	t.Setenv(EnvAppSlug, "env")
	r, _ = Resolve(Config{AppID: "global"}, Config{AppID: "dir"}, auth.Auth{}, "", "")
	if r.AppSlug != "env" {
		t.Errorf("env-wins: %q", r.AppSlug)
	}
}

// The pre-rename BITRISE_APP_SLUG env var is still honored as a fallback so
// existing shells/CI keep working; the current BITRISE_APP_ID wins over it.
func TestResolve_LegacyAppSlugEnvFallback(t *testing.T) {
	clearEnv(t)

	// Legacy env var beats config files (env > config).
	t.Setenv(EnvAppSlugLegacy, "legacy-env")
	r, _ := Resolve(Config{AppID: "global"}, Config{}, auth.Auth{}, "", "")
	if r.AppSlug != "legacy-env" {
		t.Errorf("legacy-env fallback: AppSlug = %q, want legacy-env", r.AppSlug)
	}

	// Current env var wins when both are set.
	t.Setenv(EnvAppSlug, "new-env")
	r, _ = Resolve(Config{AppID: "global"}, Config{}, auth.Auth{}, "", "")
	if r.AppSlug != "new-env" {
		t.Errorf("new env wins over legacy: AppSlug = %q, want new-env", r.AppSlug)
	}
}

func TestResolve_TokenPrecedence(t *testing.T) {
	clearEnv(t)

	// auth.yaml is the file source for tokens; config.yaml token field
	// was removed when the dedicated auth surface landed.
	r, _ := Resolve(Config{}, Config{}, auth.Auth{Token: "from-auth"}, "", "")
	if r.Token != "from-auth" {
		t.Errorf("auth-only: %q", r.Token)
	}

	// env wins over auth.yaml.
	t.Setenv(EnvToken, "from-env")
	r, _ = Resolve(Config{}, Config{}, auth.Auth{Token: "from-auth"}, "", "")
	if r.Token != "from-env" {
		t.Errorf("env-wins: %q", r.Token)
	}

	// Nothing set anywhere → empty token.
	t.Setenv(EnvToken, "")
	r, _ = Resolve(Config{}, Config{}, auth.Auth{}, "", "")
	if r.Token != "" {
		t.Errorf("none: %q", r.Token)
	}
}

func TestResolve_APIBaseURLPrecedence(t *testing.T) {
	clearEnv(t)

	// Default when nothing set.
	r, _ := Resolve(Config{}, Config{}, auth.Auth{}, "", "")
	if r.APIBaseURL != DefaultAPIBaseURL {
		t.Errorf("default: %q", r.APIBaseURL)
	}

	// Global beats default.
	r, _ = Resolve(Config{APIBaseURL: "https://global.test"}, Config{}, auth.Auth{}, "", "")
	if r.APIBaseURL != "https://global.test" {
		t.Errorf("global: %q", r.APIBaseURL)
	}

	// Env beats global.
	t.Setenv(EnvAPIBaseURL, "https://env.test")
	r, _ = Resolve(Config{APIBaseURL: "https://global.test"}, Config{}, auth.Auth{}, "", "")
	if r.APIBaseURL != "https://env.test" {
		t.Errorf("env: %q", r.APIBaseURL)
	}
}

func TestResolve_RejectsInvalidOutputFlag(t *testing.T) {
	clearEnv(t)
	_, err := Resolve(Config{}, Config{}, auth.Auth{}, "yaml", "")
	if err == nil {
		t.Fatal("expected error for invalid --output value")
	}
}

func TestResolve_ThemePrecedence(t *testing.T) {
	clearEnv(t)
	global := Config{Theme: "auto"}
	dir := Config{Theme: "dark"}
	t.Setenv(EnvTheme, "light")

	// Flag wins over env, dir, and global.
	r, err := Resolve(global, dir, auth.Auth{}, "", "none")
	if err != nil {
		t.Fatal(err)
	}
	if r.Theme != style.ThemeNone {
		t.Errorf("flag-wins: Theme = %q, want none", r.Theme)
	}

	// No flag → env beats dir + global.
	r, _ = Resolve(global, dir, auth.Auth{}, "", "")
	if r.Theme != style.ThemeLight {
		t.Errorf("env-wins: Theme = %q, want light", r.Theme)
	}

	// Clear env → dir beats global.
	t.Setenv(EnvTheme, "")
	r, _ = Resolve(global, dir, auth.Auth{}, "", "")
	if r.Theme != style.ThemeDark {
		t.Errorf("dir-wins: Theme = %q, want dark", r.Theme)
	}

	// Clear dir → global wins.
	r, _ = Resolve(global, Config{}, auth.Auth{}, "", "")
	if r.Theme != style.ThemeAuto {
		t.Errorf("global-wins: Theme = %q, want auto", r.Theme)
	}

	// Nothing set anywhere → auto.
	r, _ = Resolve(Config{}, Config{}, auth.Auth{}, "", "")
	if r.Theme != style.ThemeAuto {
		t.Errorf("default: Theme = %q, want auto", r.Theme)
	}
}

func TestResolve_RejectsInvalidThemeFlag(t *testing.T) {
	clearEnv(t)
	_, err := Resolve(Config{}, Config{}, auth.Auth{}, "", "neon")
	if err == nil {
		t.Fatal("expected error for invalid --theme value")
	}
}

func TestContext_RoundTrip(t *testing.T) {
	r := Resolved{Output: output.JSON, AppSlug: "abc"}
	ctx := WithResolved(t.Context(), r)
	got := FromContext(ctx)
	if got != r {
		t.Fatalf("round-trip: got %+v, want %+v", got, r)
	}
}

func TestFromContext_ZeroWhenAbsent(t *testing.T) {
	got := FromContext(t.Context())
	if got != (Resolved{}) {
		t.Fatalf("expected zero Resolved, got %+v", got)
	}
}
