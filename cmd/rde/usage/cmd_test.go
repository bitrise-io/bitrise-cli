package usage

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

func run(t *testing.T, c *cobra.Command, srvURL, workspaceID string, format output.Format) (string, string, error) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(stderr)
	c.SetArgs(nil)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		RDEAPIBaseURL: srvURL,
		Token:         "tok",
		Output:        format,
		WorkspaceID:   workspaceID,
	}))
	err := c.Execute()
	return stdout.String(), stderr.String(), err
}

// usageServer serves GET /v1/workspaces/ws-1/usage with a fixed body/status.
func usageServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/ws-1/usage" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

// sparseReport mimics the backend's proto3 JSON: zero-valued fields and empty
// bucket objects omitted. Alice runs 1 linux (2 vCPU / 8 GB) + 1 macos
// (4 / 6); the workspace bucket runs 1 session of unknown machine type.
const sparseReport = `{
	"totals": {
		"linux": {"sessionCount": 1, "vcpu": 2, "memoryGb": 8},
		"macos": {"sessionCount": 1, "vcpu": 4, "memoryGb": 6},
		"unknown": {"sessionCount": 1}
	},
	"users": [
		{
			"userId": "u-1", "userSlug": "alice-slug", "email": "alice@example.com", "username": "alice",
			"totals": {
				"linux": {"sessionCount": 1, "vcpu": 2, "memoryGb": 8},
				"macos": {"sessionCount": 1, "vcpu": 4, "memoryGb": 6}
			}
		},
		{
			"isWorkspace": true,
			"totals": {"unknown": {"sessionCount": 1}}
		}
	],
	"unknownMachineTypeCount": 1
}`

func TestUsageCmd_Human(t *testing.T) {
	srv := usageServer(t, http.StatusOK, sparseReport)
	defer srv.Close()

	stdout, _, err := run(t, NewCmd(), srv.URL, "ws-1", output.Human)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		"Sessions:", "3", "(Linux 1, macOS 1, unknown 1)",
		"vCPU:", "6", "(Linux 2, macOS 4)",
		"Memory GB:", "14", "(Linux 8, macOS 6)",
		"alice@example.com", "(workspace)",
		"2 / 8", "4 / 6",
		"Note: 1 active session(s) have an unrecognized machine type",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestUsageCmd_JSON_Densified(t *testing.T) {
	srv := usageServer(t, http.StatusOK, sparseReport)
	defer srv.Close()

	stdout, _, err := run(t, NewCmd(), srv.URL, "ws-1", output.JSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}

	totals, ok := got["totals"].(map[string]any)
	if !ok {
		t.Fatalf("totals missing: %s", stdout)
	}
	// The wire omitted vcpu/memoryGb on the unknown bucket; the stable shape
	// promises them as explicit zeros.
	unknown, ok := totals["unknown"].(map[string]any)
	if !ok {
		t.Fatalf("totals.unknown missing: %s", stdout)
	}
	for key, want := range map[string]float64{"session_count": 1, "vcpu": 0, "memory_gb": 0} {
		if unknown[key] != want {
			t.Errorf("totals.unknown[%q] = %v, want %v", key, unknown[key], want)
		}
	}

	users, ok := got["users"].([]any)
	if !ok || len(users) != 2 {
		t.Fatalf("users = %v, want 2 rows", got["users"])
	}
	alice := users[0].(map[string]any)
	if alice["user_slug"] != "alice-slug" {
		t.Errorf("users[0].user_slug = %v, want alice-slug", alice["user_slug"])
	}
	if got["unknown_machine_type_count"] != float64(1) {
		t.Errorf("unknown_machine_type_count = %v, want 1", got["unknown_machine_type_count"])
	}
}

func TestUsageCmd_Empty(t *testing.T) {
	srv := usageServer(t, http.StatusOK, `{}`)
	defer srv.Close()

	stdout, _, err := run(t, NewCmd(), srv.URL, "ws-1", output.Human)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "No active sessions.") {
		t.Errorf("stdout = %q, want the empty-report sentence", stdout)
	}
}

func TestUsageCmd_PermissionDenied(t *testing.T) {
	srv := usageServer(t, http.StatusForbidden, `{"code":7,"message":"you are not allowed to view usage of this workspace"}`)
	defer srv.Close()

	_, _, err := run(t, NewCmd(), srv.URL, "ws-1", output.Human)
	if err == nil || !strings.Contains(err.Error(), "not allowed to view usage") {
		t.Fatalf("error = %v, want the backend's permission message", err)
	}
}
