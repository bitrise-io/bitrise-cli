package machinetype

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
		if r.URL.Path != "/v1/workspaces/ws-1/machine-types" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"machineTypes":[{"id":"m-1","name":"g2.mac","clusterName":"c1"}]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", nil, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"m-1", "g2.mac", "c1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestListCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"machineTypes":[{"id":"m-1","name":"g2.mac","clusterName":"c1"}]}`)
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
			ClusterName string `json:"cluster_name"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if len(got.Items) != 1 || got.Items[0].ID != "m-1" || got.Items[0].Name != "g2.mac" {
		t.Errorf("unexpected JSON items: %+v", got.Items)
	}
}

func TestListCmd_EmptyHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"machineTypes":[]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", nil, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "No machine types found.") {
		t.Errorf("expected empty-state message, got: %q", stdout)
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
	if err := c.Execute(); err == nil || !strings.Contains(err.Error(), "workspace") {
		t.Errorf("error = %v, want it to mention workspace", err)
	}
}
