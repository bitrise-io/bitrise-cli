package template

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

// run drives c against a Resolved context pointing at srvURL, with
// workspaceID seeded so commands resolve it without a --workspace flag.
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
		if r.URL.Path != "/v1/workspaces/ws-1/templates" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"templates":[
			{"id":"t-1","name":"Linux Dev","image":"ubuntu","machineType":"standard","createdByEmail":"a@b.io"}
		]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", nil, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"Linux Dev", "ubuntu", "standard", "a@b.io", "t-1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestListCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"templates":[
			{"id":"t-1","name":"Linux Dev","image":"ubuntu","machineType":"standard"}
		]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", nil, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got struct {
		Items []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			MachineType string `json:"machine_type"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if len(got.Items) != 1 || got.Items[0].ID != "t-1" || got.Items[0].MachineType != "standard" {
		t.Errorf("unexpected JSON items: %+v", got.Items)
	}
}

func TestListCmd_EmptyHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"templates":[]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", nil, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "No templates found.") {
		t.Errorf("expected empty-state message, got: %q", stdout)
	}
}

func TestViewCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/ws-1/templates/t-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"template":{
			"id":"t-1","name":"Linux Dev","image":"ubuntu","machineType":"standard",
			"sessionInputs":[{"key":"repo","required":true,"description":"Repo to clone"}],
			"templateVariables":[{"key":"TOKEN","isSecret":true}]
		}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{"t-1"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"Linux Dev", "t-1", "ubuntu", "repo", "(required)", "TOKEN", "(secret)"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestViewCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"template":{"id":"t-1","name":"Linux Dev","machineType":"standard"}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{"t-1"}, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if got["id"] != "t-1" || got["machine_type"] != "standard" {
		t.Errorf("unexpected JSON: %v", got)
	}
}

func TestViewCmd_RequiresArg(t *testing.T) {
	_, _, err := run(t, newViewCmd(), "http://unused", "ws-1", nil, output.Human)
	if err == nil {
		t.Fatal("expected error when TEMPLATE_ID is missing")
	}
}

func TestListCmd_MissingWorkspace(t *testing.T) {
	c := newListCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		RDEAPIBaseURL: "http://unused",
		Token:         "tok",
		Output:        output.Human,
	}))
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "workspace") {
		t.Errorf("error = %v, want it to mention workspace", err)
	}
}
