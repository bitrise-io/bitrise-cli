package cmd

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bitrise-io/bitrise-cli/internal/auth"
	"github.com/bitrise-io/bitrise-cli/internal/config"
)

func TestAuthLogin_EmailMode_MintsAndSavesPAT(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/users/sign_in" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<meta name="csrf-token" content="t" />`)
		case r.URL.Path == "/users/sign_in":
			w.Header().Set("Location", "/dashboard")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{}`)
		case r.URL.Path == "/me/profile/security":
			_, _ = io.WriteString(w, `<meta name="csrf-token" content="t2" />`)
		case r.URL.Path == "/me/profile/security/user_auth_tokens":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"token":"bitpat_real","slug":"tok","api_url":"https://api.bitrise.io"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newAuthLoginCmd()
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	c.SetOut(stdoutBuf)
	c.SetErr(stderrBuf)
	c.SetIn(strings.NewReader("hunter2\n"))
	c.SetArgs([]string{"--email", "alice@example.com", "--password-stdin"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		WebBaseURL: srv.URL,
		Output:     "human",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stderrBuf.String(), "Saved access token") {
		t.Fatalf("stderr missing confirmation: %q", stderrBuf.String())
	}
	saved, err := auth.Load()
	if err != nil {
		t.Fatalf("auth.Load: %v", err)
	}
	if saved.Token != "bitpat_real" {
		t.Fatalf("saved token = %q, want bitpat_real", saved.Token)
	}
}

func TestAuthLogin_EmailMode_UnconfirmedShowsHint(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/sign_in" && r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `<meta name="csrf-token" content="t" />`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"You have to confirm your email address before continuing."}`)
	}))
	defer srv.Close()

	c := newAuthLoginCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetIn(strings.NewReader("hunter2\n"))
	c.SetArgs([]string{"--email", "alice@example.com", "--password-stdin"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		WebBaseURL: srv.URL,
		Output:     "human",
	}))

	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "verified its email") {
		t.Fatalf("expected friendly verify hint, got %v", err)
	}
	if a, _ := auth.Load(); a.Token != "" {
		t.Fatalf("token should not be saved on unconfirmed login; got %q", a.Token)
	}
}

func TestAuthLogin_EmailMode_RejectsWithToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	c := newAuthLoginCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetIn(strings.NewReader("anything\n"))
	c.SetArgs([]string{"--email", "alice@example.com", "--with-token"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		Output: "human",
	}))

	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "none of the others can be") {
		t.Fatalf("expected mutually-exclusive error, got %v", err)
	}
}

func TestAuthLogin_OAuthRejectsWithToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	c := newAuthLoginCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"--oauth", "--with-token"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{Output: "human"}))

	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "none of the others can be") {
		t.Fatalf("expected --oauth/--with-token to be mutually exclusive, got %v", err)
	}
}

func TestAuthLogin_WarnsWhenEnvTokenShadowsSavedToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(config.EnvToken, "ci-env-token") // shadows whatever login saves

	c := newAuthLoginCmd()
	stderr := &bytes.Buffer{}
	c.SetOut(io.Discard)
	c.SetErr(stderr)
	c.SetIn(strings.NewReader("bitpat_saved\n"))
	c.SetArgs([]string{"--with-token"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{Output: "human"}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if a, _ := auth.Load(); a.Token != "bitpat_saved" {
		t.Fatalf("token not saved: %q", a.Token)
	}
	out := stderr.String()
	if !strings.Contains(out, config.EnvToken) || !strings.Contains(out, "takes precedence") {
		t.Fatalf("expected a shadow warning naming BITRISE_TOKEN, got: %q", out)
	}
}

func TestAuthLogin_NoShadowWarningWhenEnvUnset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(config.EnvToken, "") // not shadowed

	c := newAuthLoginCmd()
	stderr := &bytes.Buffer{}
	c.SetOut(io.Discard)
	c.SetErr(stderr)
	c.SetIn(strings.NewReader("bitpat_saved\n"))
	c.SetArgs([]string{"--with-token"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{Output: "human"}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := stderr.String()
	if !strings.Contains(out, "Saved access token") {
		t.Fatalf("expected the save confirmation, got: %q", out)
	}
	if strings.Contains(out, "takes precedence") {
		t.Fatalf("unexpected shadow warning when BITRISE_TOKEN is unset: %q", out)
	}
}

func TestAuthLogin_DefaultNonInteractive_ReadsTokenFromStdin(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	c := newAuthLoginCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	// A strings.Reader isn't a terminal, so the default routes to token-from-stdin
	// (not the browser flow), keeping `echo "$TOKEN" | auth login` working.
	c.SetIn(strings.NewReader("bitpat_piped\n"))
	c.SetArgs(nil)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{Output: "human"}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if a, _ := auth.Load(); a.Token != "bitpat_piped" {
		t.Fatalf("saved token = %q, want bitpat_piped", a.Token)
	}
}

func TestAuthStatus_OAuthManaged_ShowsSourceAndExpiryNoToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("BITRISE_TOKEN", "") // ensure the env path is not taken

	expiry := time.Now().Add(45 * time.Minute)
	fixture := auth.Auth{ //nolint:gosec // G101: test fixture token, not a real credential
		Token:        "oauth-pat",
		TokenExpiry:  expiry,
		JWT:          "j",
		JWTExpiry:    expiry,
		RefreshToken: "r",
	}
	if err := auth.Save(fixture); err != nil {
		t.Fatalf("seed: %v", err)
	}

	c := newAuthStatusCmd()
	out := &bytes.Buffer{}
	c.SetOut(out)
	c.SetErr(io.Discard)
	c.SetArgs(nil)
	resolved := config.Resolved{Token: "oauth-pat", Output: "human"} //nolint:gosec // G101: test fixture token, not a real credential
	c.SetContext(config.WithResolved(context.Background(), resolved))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "oauth (auth file)") {
		t.Fatalf("expected oauth source label, got:\n%s", s)
	}
	if !strings.Contains(s, "Expires") {
		t.Fatalf("expected an expiry line, got:\n%s", s)
	}
	if strings.Contains(s, "oauth-pat") {
		t.Fatalf("token material must never be printed, got:\n%s", s)
	}
}
