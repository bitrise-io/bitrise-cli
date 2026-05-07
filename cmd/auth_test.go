package cmd

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
