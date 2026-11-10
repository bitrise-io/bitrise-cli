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
	"time"

	"github.com/spf13/cobra"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func run(t *testing.T, c *cobra.Command, srvURL, workspaceID string, format output.Format, args ...string) (string, string, error) {
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
	"unknownMachineTypeCount": 1,
	"computeQuotas": [
		{"family": "linux-lc01", "displayName": "x86 Linux", "usedVcpu": 2, "quotaVcpu": 64, "os": "linux"},
		{"family": "gcp-c2", "quotaVcpu": 16},
		{"family": "m2pro", "displayName": "M2 Pro", "usedVcpu": 48, "quotaVcpu": 48, "os": "macos"}
	]
}`

func TestUsageCmd_Human(t *testing.T) {
	srv := usageServer(t, http.StatusOK, sparseReport)
	defer srv.Close()

	stdout, stderr, err := run(t, NewCmd(), srv.URL, "ws-1", output.Human)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		"Sessions:", "3", "(Linux 1, macOS 1, unknown 1)",
		"vCPU:", "6", "(Linux 2, macOS 4)",
		"Memory GB:", "14", "(Linux 8, macOS 6)",
		"alice@example.com", "(workspace)",
		"2 / 8", "4 / 6",
		"UNKNOWN VCPU/GB",
		"MACHINE FAMILY", "x86 Linux", "Linux", "2 / 64", "62 left",
		"gcp-c2", "0 / 16", "16 left",
		"M2 Pro", "macOS", "48 / 48", "full",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "Warning") {
		t.Errorf("diagnostics leaked into stdout:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Warning:") || !strings.Contains(stderr, "(1 affected)") {
		t.Errorf("stderr missing the undercount warning:\n%s", stderr)
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
	// The stable shape's user_id is the Bitrise user ID (the wire's slug);
	// the backend's internal UUID (wire userId) is not exposed.
	alice := users[0].(map[string]any)
	if alice["user_id"] != "alice-slug" {
		t.Errorf("users[0].user_id = %v, want alice-slug", alice["user_id"])
	}
	if _, leaked := alice["user_slug"]; leaked {
		t.Errorf("users[0] leaks user_slug: %v", alice)
	}
	if got["unknown_machine_type_count"] != float64(1) {
		t.Errorf("unknown_machine_type_count = %v, want 1", got["unknown_machine_type_count"])
	}

	quotas, ok := got["compute_quotas"].([]any)
	if !ok || len(quotas) != 3 {
		t.Fatalf("compute_quotas = %v, want 3 rows", got["compute_quotas"])
	}
	// The wire omitted every field but family and quota on gcp-c2; the stable shape
	// fills in zeros and falls back to the family id for the display name.
	gcp := quotas[1].(map[string]any)
	for key, want := range map[string]any{"family": "gcp-c2", "display_name": "gcp-c2", "used_vcpu": float64(0), "quota_vcpu": float64(16), "os": ""} {
		if gcp[key] != want {
			t.Errorf("compute_quotas[1][%q] = %v, want %v", key, gcp[key], want)
		}
	}
}

func TestUsageCmd_JSON_EmptyQuotasIsArray(t *testing.T) {
	srv := usageServer(t, http.StatusOK, `{}`)
	defer srv.Close()

	stdout, _, err := run(t, NewCmd(), srv.URL, "ws-1", output.JSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, `"compute_quotas": []`) {
		t.Errorf("compute_quotas not an empty array:\n%s", stdout)
	}
}

func TestUsageCmd_Human_NoUnknownColumnWhenAllKnown(t *testing.T) {
	srv := usageServer(t, http.StatusOK, `{
		"totals": {"linux": {"sessionCount": 1, "vcpu": 2, "memoryGb": 8}},
		"users": [{
			"userSlug": "alice-slug", "email": "alice@example.com",
			"totals": {"linux": {"sessionCount": 1, "vcpu": 2, "memoryGb": 8}}
		}]
	}`)
	defer srv.Close()

	stdout, stderr, err := run(t, NewCmd(), srv.URL, "ws-1", output.Human)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(stdout, "UNKNOWN") || strings.Contains(stdout, "unknown") {
		t.Errorf("unknown column/split rendered with nothing unknown:\n%s", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want no diagnostics", stderr)
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
	if !strings.Contains(stdout, "vCPU quota: could not be loaded.") {
		t.Errorf("stdout = %q, want the missing-quota sentence", stdout)
	}
}

func TestUsageCmd_Human_QuotaWithoutSessions(t *testing.T) {
	srv := usageServer(t, http.StatusOK, `{
		"computeQuotas": [{"family": "m2pro", "displayName": "M2 Pro", "quotaVcpu": 48}]
	}`)
	defer srv.Close()

	stdout, _, err := run(t, NewCmd(), srv.URL, "ws-1", output.Human)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"No active sessions.", "M2 Pro", "0 / 48", "48 left"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
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

// Session IDs and the pool ID of sessionsReport.
const (
	idAPIWork  = "11111111-1111-1111-1111-111111111111"
	idIOSBuild = "22222222-2222-2222-2222-222222222222"
	idWarm     = "33333333-3333-3333-3333-333333333333"
	idPool     = "44444444-4444-4444-4444-444444444444"
)

// sessionsReport is the report of a workspace where the per-user session
// list is enabled, in the backend's proto3 JSON (lowerCamelCase, zero values
// omitted). Alice runs 1 linux + 1 macos session; the workspace row holds a
// warm session nobody claimed yet, of an unknown machine type.
const sessionsReport = `{
	"totals": {
		"linux": {"sessionCount": 1, "vcpu": 8, "memoryGb": 32},
		"macos": {"sessionCount": 1, "vcpu": 4, "memoryGb": 6},
		"unknown": {"sessionCount": 1}
	},
	"users": [
		{
			"userId": "u-1", "userSlug": "alice-slug", "email": "alice@example.com", "username": "alice",
			"totals": {
				"linux": {"sessionCount": 1, "vcpu": 8, "memoryGb": 32},
				"macos": {"sessionCount": 1, "vcpu": 4, "memoryGb": 6}
			},
			"sessions": [
				{"id": "` + idAPIWork + `", "name": "api-work", "os": "linux", "machineType": "g2.linux.large",
				 "vcpu": 8, "memoryGb": 32, "status": "SESSION_STATUS_RUNNING", "createdAt": "2026-09-01T08:00:00Z"},
				{"id": "` + idIOSBuild + `", "name": "ios-build", "os": "macos", "machineType": "g2.mac.m2pro.4c",
				 "vcpu": 4, "memoryGb": 6, "status": "SESSION_STATUS_TERMINATING", "createdAt": "2026-09-04T06:48:00Z"}
			]
		},
		{
			"isWorkspace": true,
			"totals": {"unknown": {"sessionCount": 1}},
			"sessions": [
				{"id": "` + idWarm + `", "name": "warm-1", "status": "SESSION_STATUS_STARTING",
				 "createdAt": "2026-09-04T11:18:00Z", "heldByWarmPoolId": "` + idPool + `"}
			]
		}
	],
	"computeQuotas": [{"family": "m2pro", "displayName": "M2 Pro", "usedVcpu": 4, "quotaVcpu": 48, "os": "macos"}]
}`

// The JSON output carries every session field in the stable snake_case
// shape, densified: this is what automation picks session IDs from.
func TestUsageCmd_JSON_UserSessions(t *testing.T) {
	srv := usageServer(t, http.StatusOK, sessionsReport)
	defer srv.Close()

	stdout, _, err := run(t, NewCmd(), srv.URL, "ws-1", output.JSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got struct {
		Users []struct {
			Sessions []map[string]any `json:"sessions"`
		} `json:"users"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if len(got.Users) != 2 || len(got.Users[0].Sessions) != 2 || len(got.Users[1].Sessions) != 1 {
		t.Fatalf("users/sessions shape wrong:\n%s", stdout)
	}
	// Oldest first, as the backend sends them.
	for key, want := range map[string]any{
		"id": idAPIWork, "name": "api-work", "os": "linux", "machine_type": "g2.linux.large",
		"vcpu": float64(8), "memory_gb": float64(32), "status": "running",
		"created_at": "2026-09-01T08:00:00Z", "held_by_warm_pool_id": "",
	} {
		if got.Users[0].Sessions[0][key] != want {
			t.Errorf("users[0].sessions[0][%q] = %v, want %v", key, got.Users[0].Sessions[0][key], want)
		}
	}
	if got.Users[0].Sessions[1]["status"] != "terminating" {
		t.Errorf("users[0].sessions[1].status = %v, want terminating", got.Users[0].Sessions[1]["status"])
	}
	// The wire omitted os, machineType, vcpu and memoryGb on the held
	// session; the stable shape fills them in.
	for key, want := range map[string]any{
		"id": idWarm, "os": "", "machine_type": "", "vcpu": float64(0), "memory_gb": float64(0),
		"status": "starting", "held_by_warm_pool_id": idPool,
	} {
		if got.Users[1].Sessions[0][key] != want {
			t.Errorf("users[1].sessions[0][%q] = %v, want %v", key, got.Users[1].Sessions[0][key], want)
		}
	}
}

// Where the list is not enabled the backend omits it; the stable shape still
// promises an array, so `.users[].sessions[]` never trips over null.
func TestUsageCmd_JSON_UserSessionsEmptyIsArray(t *testing.T) {
	srv := usageServer(t, http.StatusOK, sparseReport)
	defer srv.Close()

	stdout, _, err := run(t, NewCmd(), srv.URL, "ws-1", output.JSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Count(stdout, `"sessions": []`); got != 2 {
		t.Errorf("want an empty sessions array on both user rows, found %d:\n%s", got, stdout)
	}
}

// Without --sessions the human output is the report as it always was, even
// when the backend sends the list.
func TestUsageCmd_Human_SessionsHiddenByDefault(t *testing.T) {
	srv := usageServer(t, http.StatusOK, sessionsReport)
	defer srv.Close()

	stdout, _, err := run(t, NewCmd(), srv.URL, "ws-1", output.Human)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, unwanted := range []string{"api-work", idAPIWork, "HELD BY POOL", "AGE", "Session list"} {
		if strings.Contains(stdout, unwanted) {
			t.Errorf("stdout contains %q without --sessions:\n%s", unwanted, stdout)
		}
	}
	for _, want := range []string{"alice@example.com", "(workspace)", "MACHINE FAMILY"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

// The full --sessions layout against a fixed clock: the per-user lists sit
// between the per-user table and the quota table, oldest first, and the pool
// column and its hint appear only because a held session is listed.
func TestRenderUsage_Sessions(t *testing.T) {
	srv := usageServer(t, http.StatusOK, sessionsReport)
	defer srv.Close()
	res, err := internalrde.NewService(rdeapi.New(srv.URL, "tok")).GetWorkspaceUsage(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("GetWorkspaceUsage: %v", err)
	}

	var buf bytes.Buffer
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	if err := renderUsage(&buf, res, true, now); err != nil {
		t.Fatalf("renderUsage: %v", err)
	}
	const want = `Sessions:   3     (Linux 1, macOS 1, unknown 1)
vCPU:       12    (Linux 8, macOS 4)
Memory GB:  38    (Linux 32, macOS 6)

USER               SESSIONS  LINUX VCPU/GB  MACOS VCPU/GB  UNKNOWN VCPU/GB
alice@example.com  2         8 / 32         4 / 6          0 / 0
(workspace)        1         0 / 0          0 / 0          0 / 0

alice@example.com
NAME       STATUS       AGE     OS     VCPU/GB  ID
api-work   running      3d 4h   Linux  8 / 32   ` + idAPIWork + `
ios-build  terminating  5h 12m  macOS  4 / 6    ` + idIOSBuild + `

(workspace)
NAME    STATUS    AGE  OS  VCPU/GB  HELD BY POOL                          ID
warm-1  starting  42m  -   0 / 0    ` + idPool + `  ` + idWarm + `

A session held by a pool cannot be terminated directly: lower the pool's size ('rde warm-pool set-size'), or retry the pool if its setup failed ('rde warm-pool retry').

MACHINE FAMILY  OS     VCPU USED / QUOTA  STATUS
M2 Pro          macOS  4 / 48             44 left
`
	if got := buf.String(); got != want {
		t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// --sessions reaches the renderer through the flag.
func TestUsageCmd_Human_SessionsFlag(t *testing.T) {
	srv := usageServer(t, http.StatusOK, sessionsReport)
	defer srv.Close()

	stdout, _, err := run(t, NewCmd(), srv.URL, "ws-1", output.Human, "--sessions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		"NAME", "STATUS", "AGE", "VCPU/GB", "HELD BY POOL",
		"api-work", "running", idAPIWork, "ios-build", "terminating", idIOSBuild,
		"warm-1", "starting", idPool, idWarm,
		"cannot be terminated directly",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	// Oldest first within a user.
	if strings.Index(stdout, "api-work") > strings.Index(stdout, "ios-build") {
		t.Errorf("sessions not oldest first:\n%s", stdout)
	}
}

// A workspace where the list is not enabled has active sessions but none
// listed: --sessions says so instead of printing nothing.
func TestUsageCmd_Human_SessionsFlag_NotEnabled(t *testing.T) {
	srv := usageServer(t, http.StatusOK, sparseReport)
	defer srv.Close()

	stdout, _, err := run(t, NewCmd(), srv.URL, "ws-1", output.Human, "--sessions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "Session list not available: it is not enabled for this workspace") {
		t.Errorf("stdout missing the not-enabled note:\n%s", stdout)
	}
	// The rest of the report is still there.
	for _, want := range []string{"alice@example.com", "MACHINE FAMILY"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

// With nothing running there is no list to miss: no note either.
func TestUsageCmd_Human_SessionsFlag_NoActiveSessions(t *testing.T) {
	srv := usageServer(t, http.StatusOK, `{}`)
	defer srv.Close()

	stdout, _, err := run(t, NewCmd(), srv.URL, "ws-1", output.Human, "--sessions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "No active sessions.") || strings.Contains(stdout, "Session list") {
		t.Errorf("stdout = %q, want only the empty-report sentence", stdout)
	}
}

func TestFormatAge(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time {
		t := now.Add(-d)
		return &t
	}
	for _, tc := range []struct {
		created *time.Time
		want    string
	}{
		{nil, "-"},
		{ago(20 * time.Second), "<1m"},
		{ago(-5 * time.Second), "<1m"}, // clock skew: never a negative age
		{ago(42 * time.Minute), "42m"},
		{ago(time.Hour), "1h"},
		{ago(5*time.Hour + 12*time.Minute), "5h 12m"},
		{ago(24 * time.Hour), "1d"},
		{ago(24*time.Hour + 30*time.Minute), "1d"},
		{ago(76 * time.Hour), "3d 4h"},
		{ago(400 * 24 * time.Hour), "400d"},
	} {
		if got := formatAge(tc.created, now); got != tc.want {
			t.Errorf("formatAge(%v) = %q, want %q", tc.created, got, tc.want)
		}
	}
}
