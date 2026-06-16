package session

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

// run drives c with the given args and a Resolved context pointing at
// srvURL. workspaceID seeds the resolved WorkspaceID so commands resolve it
// without a --workspace flag.
func run(t *testing.T, c *cobra.Command, srvURL, workspaceID string, args []string, format output.Format) (string, string, error) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(stderr)
	c.SetArgs(args)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		RDEAPIBaseURL: srvURL,
		Token:         "tok",
		Output:        format,
		WorkspaceID:   workspaceID,
	}))
	err := c.Execute()
	return stdout.String(), stderr.String(), err
}

func TestListCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/ws-1/sessions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("auth = %q", got)
		}
		_, _ = io.WriteString(w, `{"sessions":[
			{"id":"s-1","name":"dev","status":"SESSION_STATUS_RUNNING","templateSnapshot":{"templateName":"tmpl"}}
		]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", nil, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"dev", "running", "tmpl", "s-1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestListCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"sessions":[{"id":"s-1","name":"dev","status":"SESSION_STATUS_RUNNING"}]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", nil, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got struct {
		Items []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if len(got.Items) != 1 || got.Items[0].ID != "s-1" || got.Items[0].Status != "running" {
		t.Errorf("unexpected JSON: %+v", got.Items)
	}
}

func TestListCmd_MissingWorkspace(t *testing.T) {
	// No WorkspaceID in Resolved and no --workspace flag → error before any HTTP call.
	c := newListCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		RDEAPIBaseURL: "http://unused",
		Token:         "tok",
		Output:        output.Human,
	}))
	err := c.Execute()
	if err == nil {
		t.Fatal("expected workspace-required error, got nil")
	}
	if !strings.Contains(err.Error(), "workspace") {
		t.Errorf("error = %v, want it to mention workspace", err)
	}
}

func TestCreateCmd_RequiresTemplateAndName(t *testing.T) {
	// NAME given positionally but no --template fails fast.
	_, _, err := run(t, newCreateCmd(), "http://unused", "ws-1",
		[]string{"dev"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--template") {
		t.Errorf("error = %v, want --template required", err)
	}
}

func TestParseSessionInputs(t *testing.T) {
	got, err := parseSessionInputs(
		[]string{"repo=my-app"},
		[]string{"token=ghp_x"},
		[]string{"key=saved-id"},
	)
	if err != nil {
		t.Fatalf("parseSessionInputs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d inputs, want 3", len(got))
	}
	if got[0].Key != "repo" || got[0].Value != "my-app" || got[0].IsSecret {
		t.Errorf("plain input wrong: %+v", got[0])
	}
	if !got[1].IsSecret || got[1].Value != "ghp_x" {
		t.Errorf("secret input wrong: %+v", got[1])
	}
	if got[2].SavedInputID != "saved-id" || got[2].Value != "" {
		t.Errorf("saved input wrong: %+v", got[2])
	}

	if _, err := parseSessionInputs([]string{"bad"}, nil, nil); err == nil {
		t.Error("expected error for malformed --input")
	}
	if _, err := parseSessionInputs(nil, nil, []string{"key="}); err == nil {
		t.Error("expected error for --saved-input with empty ID")
	}
}
