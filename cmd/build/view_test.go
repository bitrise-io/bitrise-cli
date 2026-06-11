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

func TestViewCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/my-app/builds/b-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"data":{"slug":"b-1","build_number":7,"status":1,"triggered_workflow":"primary","branch":"main","triggered_at":"2026-05-06T10:00:00Z"}}`)
	}))
	defer srv.Close()

	c := newViewCmd()
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
	for _, want := range []string{"b-1", "#7", "success", "primary", "main"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestViewCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"slug":"b-2","build_number":8,"status":2,"triggered_workflow":"deploy","branch":"feature","triggered_at":"2026-05-06T10:00:00Z"}}`)
	}))
	defer srv.Close()

	c := newViewCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"b-2"})
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
	if got["id"] != "b-2" || got["status"] != "failed" {
		t.Errorf("unexpected JSON: %v", got)
	}
}

func TestViewCmd_MissingArg(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	c := newViewCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "BUILD_ID") {
		t.Errorf("expected missing BUILD_ID error, got %v", err)
	}
}
