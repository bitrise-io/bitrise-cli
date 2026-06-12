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

	"github.com/bitrise-io/bitrise-cli/cmd/cmdtest"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func TestBuildYMLCmd_HappyPath(t *testing.T) {
	const ymlContent = "format_version: \"13\"\nworkflows:\n  primary:\n    steps: []\n"
	srv := httptest.NewServer(cmdtest.AppPassthrough(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/my-app/builds/b-1/bitrise.yml" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, ymlContent)
	}))
	defer srv.Close()

	c := newYMLCmd()
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
	if !strings.Contains(stdout.String(), "format_version") {
		t.Errorf("stdout missing yml content: %q", stdout.String())
	}
}

func TestBuildYMLCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(cmdtest.AppPassthrough(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "format_version: \"13\"\n")
	}))
	defer srv.Close()

	c := newYMLCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"b-1"})
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
	if got["app_id"] != "my-app" || got["build_id"] != "b-1" {
		t.Errorf("unexpected JSON: %v", got)
	}
	if _, ok := got["content"]; !ok {
		t.Errorf("JSON missing 'content' field: %v", got)
	}
}
