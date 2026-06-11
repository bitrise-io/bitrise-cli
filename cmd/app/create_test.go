package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-cli/internal/config"
)

// stubAPI multiplexes register/finish/upload/organizations endpoints for
// the cmd-level happy-path test. Bodies are recorded for assertions.
type stubAPI struct {
	t      *testing.T
	srv    *httptest.Server
	bodies map[string][]byte
}

func newStubAPI(t *testing.T, handlers map[string]http.HandlerFunc) *stubAPI {
	t.Helper()
	s := &stubAPI{t: t, bodies: map[string][]byte{}}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.bodies[r.URL.Path] = body
		h, ok := handlers[r.URL.Path]
		if !ok {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func TestCreateCmd_HappyPath_PersistsAppSlug(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	api := newStubAPI(t, map[string]http.HandlerFunc{
		"/apps/register": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"status":"ok","slug":"created-app"}`)
		},
		"/apps/created-app/finish": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"status":"ok","build_trigger_token":"tok-123","branch_name":"main"}`)
		},
	})

	c := newCreateCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(stderr)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		Output:     "human",
		APIBaseURL: api.srv.URL,
		Token:      "tok",
	}))
	c.SetArgs([]string{
		"--repo-url", "https://github.com/acme/widget.git",
		"--workspace", "acme",
		"--branch", "main",
		"--title", "Widget",
	})

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// stdout shows the result.
	out := stdout.String()
	for _, want := range []string{"Created app created-app", "Widget", "https://github.com/acme/widget.git", "tok-123"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\n--- stdout ---\n%s", want, out)
		}
	}

	// app_id got saved to the temp XDG config.
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.AppID != "created-app" {
		t.Errorf("config.AppSlug = %q, want created-app", cfg.AppID)
	}

	// stderr shows the persist message.
	if !strings.Contains(stderr.String(), "Set app_id=created-app") {
		t.Errorf("stderr missing persist log:\n%s", stderr.String())
	}

	// We sent the right register body.
	var reg map[string]any
	_ = json.Unmarshal(api.bodies["/apps/register"], &reg)
	if reg["organization_slug"] != "acme" || reg["provider"] != "custom" {
		t.Errorf("register body = %v", reg)
	}
}

func TestCreateCmd_AutoOrgFromConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	api := newStubAPI(t, map[string]http.HandlerFunc{
		"/apps/register": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"status":"ok","slug":"x"}`)
		},
		"/apps/x/finish": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"status":"ok","build_trigger_token":"t","branch_name":"main"}`)
		},
	})

	c := newCreateCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		Output:     "human",
		APIBaseURL: api.srv.URL,
		Token:      "tok",
		OrgSlug:    "from-config",
	}))
	c.SetArgs([]string{
		"--repo-url", "https://github.com/a/b.git",
		"--branch", "main",
	})

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var reg map[string]any
	_ = json.Unmarshal(api.bodies["/apps/register"], &reg)
	if reg["organization_slug"] != "from-config" {
		t.Errorf("organization_slug = %v, want from-config", reg["organization_slug"])
	}
}

func TestCreateCmd_JSONOutput(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	api := newStubAPI(t, map[string]http.HandlerFunc{
		"/apps/register": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"status":"ok","slug":"json-app"}`)
		},
		"/apps/json-app/finish": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"status":"ok","build_trigger_token":"jbt","branch_name":"main"}`)
		},
	})

	c := newCreateCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		Output:     "json",
		APIBaseURL: api.srv.URL,
		Token:      "tok",
	}))
	c.SetArgs([]string{
		"--repo-url", "https://github.com/a/b.git",
		"--workspace", "acme",
		"--branch", "main",
	})

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON on stdout: %v\n%s", err, stdout.String())
	}
	if got["id"] != "json-app" || got["build_trigger_token"] != "jbt" {
		t.Errorf("JSON = %v", got)
	}
}
