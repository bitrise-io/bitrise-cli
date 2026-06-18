package cmdutil

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

// wsCmd builds a command with the --workspace flag and the given Resolved on
// its context, mirroring how persistentPreRunE seeds real commands.
func wsCmd(t *testing.T, r config.Resolved) *cobra.Command {
	t.Helper()
	c := &cobra.Command{RunE: func(*cobra.Command, []string) error { return nil }}
	c.Flags().String(FlagWorkspace, "", "")
	c.SetContext(config.WithResolved(context.Background(), r))
	return c
}

// orgsServer serves the given GET /organizations body and fails the test if
// any other path is hit.
func orgsServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/organizations" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestResolveWorkspaceID_FlagWins(t *testing.T) {
	c := wsCmd(t, config.Resolved{WorkspaceID: "from-ctx"})
	if err := c.Flags().Set(FlagWorkspace, "from-flag"); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveWorkspaceID(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-flag" {
		t.Errorf("got %q, want from-flag", got)
	}
}

func TestResolveWorkspaceID_ContextWins(t *testing.T) {
	c := wsCmd(t, config.Resolved{WorkspaceID: "from-ctx"})
	got, err := ResolveWorkspaceID(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-ctx" {
		t.Errorf("got %q, want from-ctx", got)
	}
}

func TestResolveWorkspaceID_AutoDetectSingle(t *testing.T) {
	t.Setenv(config.EnvToken, "tok")
	srv := orgsServer(t, `{"data":[{"slug":"solo","name":"Solo"}],"paging":{}}`)

	c := wsCmd(t, config.Resolved{APIBaseURL: srv.URL, Output: output.Human})
	var stderr bytes.Buffer
	c.SetErr(&stderr)

	got, err := ResolveWorkspaceID(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "solo" {
		t.Errorf("got %q, want solo", got)
	}
	if !strings.Contains(stderr.String(), "Using your only workspace: Solo (solo)") {
		t.Errorf("missing auto-detect breadcrumb, stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "config set default_workspace_id solo") {
		t.Errorf("missing persist hint, stderr=%q", stderr.String())
	}
}

// In JSON mode the auto-pick still works but emits no human breadcrumbs —
// scripts/CI shouldn't get hint noise on stderr.
func TestResolveWorkspaceID_AutoDetectJSONIsSilent(t *testing.T) {
	t.Setenv(config.EnvToken, "tok")
	srv := orgsServer(t, `{"data":[{"slug":"solo","name":"Solo"}],"paging":{}}`)

	c := wsCmd(t, config.Resolved{APIBaseURL: srv.URL, Output: output.JSON})
	var stderr bytes.Buffer
	c.SetErr(&stderr)

	got, err := ResolveWorkspaceID(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "solo" {
		t.Errorf("got %q, want solo", got)
	}
	if stderr.String() != "" {
		t.Errorf("expected no stderr output in JSON mode, got %q", stderr.String())
	}
}

func TestResolveWorkspaceID_AutoDetectMultipleErrors(t *testing.T) {
	t.Setenv(config.EnvToken, "tok")
	srv := orgsServer(t, `{"data":[{"slug":"a"},{"slug":"b"}],"paging":{}}`)

	c := wsCmd(t, config.Resolved{APIBaseURL: srv.URL})
	_, err := ResolveWorkspaceID(c)
	if err == nil || !strings.Contains(err.Error(), "multiple workspaces") {
		t.Fatalf("expected multiple-workspaces error, got %v", err)
	}
}
