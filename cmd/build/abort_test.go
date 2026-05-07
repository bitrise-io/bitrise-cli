package build

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
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func TestAbortCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/my-app/builds/b-1/abort" || r.Method != http.MethodPost {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer srv.Close()

	c := newAbortCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"b-1"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Build aborted") || !strings.Contains(out, "b-1") {
		t.Errorf("stdout missing expected text: %q", out)
	}
}

func TestAbortCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer srv.Close()

	c := newAbortCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"b-1", "--reason", "no longer needed"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.JSON,
		AppSlug:    "my-app",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if got["build_slug"] != "b-1" || got["status"] != "ok" {
		t.Errorf("unexpected JSON: %v", got)
	}
}

func TestAbortCmd_SendsReasonInBody(t *testing.T) {
	var gotReason string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotReason, _ = body["abort_reason"].(string)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer srv.Close()

	c := newAbortCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"b-1", "--reason", "cleanup"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotReason != "cleanup" {
		t.Errorf("abort_reason = %q, want cleanup", gotReason)
	}
}
