package step

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

func TestInputsCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/step-inputs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if !strings.Contains(r.URL.RawQuery, "step_ref=git-clone%408.3.1") {
			t.Errorf("expected step_ref in %q", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `[
			{"name":"repository_url","title":"Repository URL","is_required":true},
			{"name":"clone_into_dir","title":"Clone into directory","default_value":".","is_required":false}
		]`)
	}))
	defer srv.Close()

	c := newInputsCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"git-clone@8.3.1"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"repository_url", "Repository URL", "clone_into_dir"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestInputsCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"name":"repository_url","title":"Repository URL","is_required":true}]`)
	}))
	defer srv.Close()

	c := newInputsCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"git-clone@8.3.1"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.JSON,
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if got["step_ref"] != "git-clone@8.3.1" {
		t.Errorf("expected step_ref 'git-clone@8.3.1', got %v", got["step_ref"])
	}
	items, _ := got["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestInputsCmd_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()

	c := newInputsCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"some-step@1.0.0"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "No inputs found") {
		t.Errorf("expected empty-state message, got: %q", stdout.String())
	}
}
