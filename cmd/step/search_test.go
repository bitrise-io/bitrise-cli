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

func TestSearchCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search-steps" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if !strings.Contains(r.URL.RawQuery, "query=clone") {
			t.Errorf("expected query=clone in %q", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `[{"id":"git-clone","step_ref":"git-clone@8.3.1","title":"Git Clone Repository","maintainer":"bitrise","summary":"Clones a Git repository"}]`)
	}))
	defer srv.Close()

	c := newSearchCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"clone"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"git-clone@8.3.1", "Git Clone Repository", "bitrise"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestSearchCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"id":"git-clone","step_ref":"git-clone@8.3.1","title":"Git Clone Repository","maintainer":"bitrise"}]`)
	}))
	defer srv.Close()

	c := newSearchCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"clone"})
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
	items, _ := got["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item, _ := items[0].(map[string]any)
	if item["step_ref"] != "git-clone@8.3.1" {
		t.Errorf("unexpected item: %v", item)
	}
}

func TestSearchCmd_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()

	c := newSearchCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"zzz-nonexistent"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "No steps found") {
		t.Errorf("expected empty-state message, got: %q", stdout.String())
	}
}
