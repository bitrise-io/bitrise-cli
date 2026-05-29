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

// catalogServer returns a test server that serves the two upstream endpoints
// (/images and /machine-types) the command joins on.
func catalogServer(t *testing.T, imagesJSON, machineTypesJSON string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/workspaces/ws-1/images":
			_, _ = io.WriteString(w, imagesJSON)
		case "/v1/workspaces/ws-1/machine-types":
			_, _ = io.WriteString(w, machineTypesJSON)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

func TestListCmd_RequiresImageFlag(t *testing.T) {
	_, _, err := run(t, newListCmd(), "http://unused", "ws-1", nil, output.Human)
	if err == nil || !strings.Contains(err.Error(), "image") {
		t.Fatalf("error = %v, want it to mention required --image flag", err)
	}
}

func TestListCmd_HidesClusterWhenUnambiguous(t *testing.T) {
	srv := catalogServer(t,
		`{"images":[{"id":"i-1","name":"osx-xcode","clusterName":"c1"}]}`,
		`{"machineTypes":[
			{"id":"m-1","name":"g2.mac.m2pro.4c","clusterName":"c1"},
			{"id":"m-2","name":"g2.mac.m1.8c","clusterName":"c1"}
		]}`,
	)
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", []string{"--image", "osx-xcode"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"g2.mac.m2pro.4c", "g2.mac.m1.8c"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "CLUSTER") || strings.Contains(stdout, "c1") {
		t.Errorf("unambiguous case should hide CLUSTER column, got:\n%s", stdout)
	}
}

func TestListCmd_ShowsClusterWhenAmbiguous(t *testing.T) {
	srv := catalogServer(t,
		`{"images":[
			{"id":"i-1","name":"osx-xcode","clusterName":"c1"},
			{"id":"i-2","name":"osx-xcode","clusterName":"c2"}
		]}`,
		`{"machineTypes":[
			{"id":"m-1","name":"g2.mac.m2pro.4c","clusterName":"c1"},
			{"id":"m-2","name":"g2.mac.m2pro.4c","clusterName":"c2"},
			{"id":"m-3","name":"g2.mac.m1.8c","clusterName":"c1"}
		]}`,
	)
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", []string{"--image", "osx-xcode"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "CLUSTER") {
		t.Errorf("ambiguous case should show CLUSTER column, got:\n%s", stdout)
	}
	for _, want := range []string{"c1", "c2"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing cluster %q:\n%s", want, stdout)
		}
	}
}

func TestListCmd_FiltersByImageCluster(t *testing.T) {
	srv := catalogServer(t,
		`{"images":[{"id":"i-1","name":"osx-xcode","clusterName":"c1"}]}`,
		`{"machineTypes":[
			{"id":"m-1","name":"g2.mac.m2pro.4c","clusterName":"c1"},
			{"id":"m-2","name":"g3.linux.8c","clusterName":"c2"}
		]}`,
	)
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", []string{"--image", "osx-xcode"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "g2.mac.m2pro.4c") {
		t.Errorf("stdout missing matching MT:\n%s", stdout)
	}
	if strings.Contains(stdout, "g3.linux.8c") {
		t.Errorf("stdout should have excluded MT from a non-matching cluster, got:\n%s", stdout)
	}
}

func TestListCmd_UnknownImage(t *testing.T) {
	srv := catalogServer(t,
		`{"images":[{"id":"i-1","name":"osx-xcode","clusterName":"c1"}]}`,
		`{"machineTypes":[]}`,
	)
	defer srv.Close()

	_, _, err := run(t, newListCmd(), srv.URL, "ws-1", []string{"--image", "does-not-exist"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want it to mention image not found", err)
	}
}

func TestListCmd_JSONOutput(t *testing.T) {
	srv := catalogServer(t,
		`{"images":[{"id":"i-1","name":"osx-xcode","clusterName":"c1"}]}`,
		`{"machineTypes":[{"id":"m-1","name":"g2.mac","clusterName":"c1"}]}`,
	)
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", []string{"--image", "osx-xcode"}, output.JSON)
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
	srv := catalogServer(t,
		`{"images":[{"id":"i-1","name":"osx-xcode","clusterName":"c1"}]}`,
		`{"machineTypes":[]}`,
	)
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", []string{"--image", "osx-xcode"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "No machine types available") {
		t.Errorf("expected empty-state message, got: %q", stdout)
	}
}

func TestListCmd_MissingWorkspace(t *testing.T) {
	c := newListCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"--image", "osx-xcode"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		RDEAPIBaseURL: "http://unused",
		Token:         "tok",
		Output:        output.Human,
	}))
	if err := c.Execute(); err == nil || !strings.Contains(err.Error(), "workspace") {
		t.Errorf("error = %v, want it to mention workspace", err)
	}
}
