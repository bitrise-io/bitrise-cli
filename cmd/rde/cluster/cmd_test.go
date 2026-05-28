package cluster

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

func TestResolveCmd_RequiresImage(t *testing.T) {
	_, _, err := run(t, newResolveCmd(), "http://unused", "ws-1",
		[]string{"--machine-type", "g2.mac"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--image") {
		t.Errorf("error = %v, want --image required", err)
	}
}

func TestResolveCmd_RequiresMachineType(t *testing.T) {
	_, _, err := run(t, newResolveCmd(), "http://unused", "ws-1",
		[]string{"--image", "osx-xcode"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--machine-type") {
		t.Errorf("error = %v, want --machine-type required", err)
	}
}

func TestResolveCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/workspaces/ws-1/resolve-clusters" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["image"] != "osx-xcode-edge" || body["machineType"] != "g2.mac" {
			t.Errorf("unexpected resolve body: %v", body)
		}
		_, _ = io.WriteString(w, `{"clusters":[{"clusterName":"c1","imageId":"i","machineTypeId":"m"}]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newResolveCmd(), srv.URL, "ws-1",
		[]string{"--image", "osx-xcode-edge", "--machine-type", "g2.mac"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "c1") {
		t.Errorf("stdout missing cluster name:\n%s", stdout)
	}
}

func TestResolveCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"clusters":[{"clusterName":"c1","imageId":"i-1","machineTypeId":"m-1"}]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newResolveCmd(), srv.URL, "ws-1",
		[]string{"--image", "x", "--machine-type", "y"}, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got struct {
		Items []struct {
			ClusterName   string `json:"cluster_name"`
			ImageID       string `json:"image_id"`
			MachineTypeID string `json:"machine_type_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if len(got.Items) != 1 || got.Items[0].ClusterName != "c1" || got.Items[0].MachineTypeID != "m-1" {
		t.Errorf("unexpected JSON items: %+v", got.Items)
	}
}

func TestResolveCmd_EmptyHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"clusters":[]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newResolveCmd(), srv.URL, "ws-1",
		[]string{"--image", "x", "--machine-type", "y"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "No clusters serve") {
		t.Errorf("expected empty-state message, got: %q", stdout)
	}
}
