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

func TestListCmd_LabelSelectorsQuery(t *testing.T) {
	var gotSelectors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSelectors = r.URL.Query()["labelSelectors"]
		_, _ = io.WriteString(w, `{"sessions":[]}`)
	}))
	defer srv.Close()

	_, _, err := run(t, newListCmd(), srv.URL, "ws-1",
		[]string{"-l", "team=mobile", "--label-selector", "branch=main"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(gotSelectors) != 2 || gotSelectors[0] != "team=mobile" || gotSelectors[1] != "branch=main" {
		t.Errorf("labelSelectors = %v, want [team=mobile branch=main]", gotSelectors)
	}
}

func TestListCmd_ScopeQuery(t *testing.T) {
	var gotScope string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotScope = r.URL.Query().Get("scope")
		_, _ = io.WriteString(w, `{"sessions":[]}`)
	}))
	defer srv.Close()

	// Default scope is mine, sent as the backend's enum name.
	if _, _, err := run(t, newListCmd(), srv.URL, "ws-1", nil, output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotScope != "SESSION_LIST_SCOPE_MINE" {
		t.Errorf("default scope = %q, want SESSION_LIST_SCOPE_MINE", gotScope)
	}

	if _, _, err := run(t, newListCmd(), srv.URL, "ws-1", []string{"--scope", "workspace"}, output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotScope != "SESSION_LIST_SCOPE_WORKSPACE" {
		t.Errorf("scope = %q, want SESSION_LIST_SCOPE_WORKSPACE", gotScope)
	}
}

func TestListCmd_InvalidScopeErrors(t *testing.T) {
	// Validation happens before any HTTP call.
	_, _, err := run(t, newListCmd(), "http://unused", "ws-1",
		[]string{"--scope", "everyone"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--scope") {
		t.Errorf("error = %v, want scope validation error", err)
	}
}

func TestListCmd_MalformedSelectorErrors(t *testing.T) {
	_, _, err := run(t, newListCmd(), "http://unused", "ws-1",
		[]string{"-l", "bare-key"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--label-selector") {
		t.Errorf("error = %v, want selector parse error", err)
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

// TestDiffCmd_ShowsDeviceChange: a template that gained (or changed) its
// declared device shows up as a "Device:" line like stack/machine type do.
func TestDiffCmd_ShowsDeviceChange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/ws-1/sessions/"+uuidSession+"/template-diff" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{
			"snapshot":{"stackId":"osx-xcode-16.0.x-edge","machineType":"g2.mac"},
			"current":{"stackId":"osx-xcode-16.0.x-edge","machineType":"g2.mac","deviceSpec":{"platform":"ios","deviceModel":"iPhone 16","osVersion":"18.2"}}
		}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newDiffCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "Device:") || !strings.Contains(stdout, "(unset) → iOS simulator · iPhone 16 · 18.2") {
		t.Errorf("stdout missing the device change:\n%s", stdout)
	}
	if strings.Contains(stdout, "Stack:") {
		t.Errorf("unchanged stack must not be listed:\n%s", stdout)
	}
}

func TestViewCmd_ShowsLifecycle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/ws-1/sessions/"+uuidSession {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_RUNNING",
			"lifecycle":{"mode":"hooks","phase":"LIFECYCLE_PHASE_SETUP_FAILED","hasOnRestoreHook":true,
				"runs":[
					{"id":"r-1","hook":"on_create","attempt":1,"state":"HOOK_RUN_STATE_FAILED","exitCode":2,"triggeredBy":"create","ranIn":"session","queuedAt":"2026-10-07T10:00:00Z","startedAt":"2026-10-07T10:00:05Z","finishedAt":"2026-10-07T10:01:00Z"},
					{"id":"r-2","hook":"personalize","attempt":1,"state":"HOOK_RUN_STATE_SKIPPED","triggeredBy":"create","ranIn":"session","queuedAt":"2026-10-07T10:00:00Z"}
				]},
			"templateSnapshot":{"templateName":"tmpl","lifecycleMode":"hooks","hasOnCreateHook":true,"hasOnRestoreHook":true,
				"sessionInputs":[{"key":"GH_TOKEN","isSecret":true,"hook":"on_claim"}]}}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"Setup:", "setup failed", "Lifecycle hooks", "On create #1:", "failed (exit 2)", "System setup #1:", "skipped"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}

	stdout, _, err = run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.JSON)
	if err != nil {
		t.Fatalf("Execute (json): %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	lc, _ := got["lifecycle"].(map[string]any)
	if lc["mode"] != "hooks" || lc["phase"] != "setup_failed" || lc["has_on_restore_hook"] != true {
		t.Errorf("unexpected lifecycle in JSON: %v", got["lifecycle"])
	}
	runs, _ := lc["runs"].([]any)
	if len(runs) != 2 {
		t.Fatalf("runs = %v, want 2", lc["runs"])
	}
	first, _ := runs[0].(map[string]any)
	if first["hook"] != "on_create" || first["state"] != "failed" || first["exit_code"] != float64(2) || first["started_at"] == nil {
		t.Errorf("unexpected first run in JSON: %v", first)
	}
	// The ready signal is gone: a run has no ready_via / ready_at.
	for _, gone := range []string{"ready_via", "ready_at"} {
		if _, ok := first[gone]; ok {
			t.Errorf("run must not carry %s: %v", gone, first)
		}
	}
	snap, _ := got["template_snapshot"].(map[string]any)
	if snap["lifecycle_mode"] != "hooks" || snap["has_on_create_hook"] != true || snap["has_on_restore_hook"] != true {
		t.Errorf("unexpected snapshot lifecycle fields in JSON: %v", snap)
	}
	inputs, _ := snap["session_inputs"].([]any)
	if in, _ := inputs[0].(map[string]any); in["hook"] != "on_claim" {
		t.Errorf("session_inputs[0].hook = %v, want on_claim", in["hook"])
	}
}

func TestViewCmd_LegacySessionHasNoLifecycle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_RUNNING"}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Contains(stdout, "Setup:") || strings.Contains(stdout, "Lifecycle hooks") {
		t.Errorf("a legacy session must not render lifecycle lines:\n%s", stdout)
	}
	stdout, _, err = run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.JSON)
	if err != nil {
		t.Fatalf("Execute (json): %v", err)
	}
	if strings.Contains(stdout, `"lifecycle"`) {
		t.Errorf("--output json must omit lifecycle on a legacy session:\n%s", stdout)
	}
}

func TestDiffCmd_ShowsLifecycleHooksChange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{
			"snapshot":{"stackId":"linux","machineType":"standard","lifecycleMode":"hooks","hooks":{"onCreate":"echo create"}},
			"current":{"stackId":"linux","machineType":"standard","lifecycleMode":"hooks","hooks":{"onCreate":"echo create","onClaim":"echo claim"}}
		}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newDiffCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "Lifecycle:") || !strings.Contains(stdout, "hooks (On create) → hooks (On create, On claim)") {
		t.Errorf("stdout missing the lifecycle change:\n%s", stdout)
	}
}

// A legacy template's session carries platform-mapped hooks on the snapshot
// side; that is not a template edit and must not read as a difference.
func TestDiffCmd_MappedLegacySnapshotIsNoDifference(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{
			"snapshot":{"stackId":"linux","machineType":"standard","startupScript":"npm run dev","lifecycleMode":"hooks","hooks":{"onCreate":"# Converted","onRestore":"npm run dev"}},
			"current":{"stackId":"linux","machineType":"standard","startupScript":"npm run dev","lifecycleMode":"legacy"}
		}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newDiffCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "No differences.") {
		t.Errorf("stdout should report no differences:\n%s", stdout)
	}
}

// An On create that RDE ended at its limit (exit 124) is explained on the
// run line: the script has to exit, long-running work goes to the background.
func TestViewCmd_OnCreateLimitExplained(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_RUNNING",
			"lifecycle":{"mode":"hooks","phase":"LIFECYCLE_PHASE_SETUP_FAILED",
				"runs":[{"id":"r-1","hook":"on_create","attempt":1,"state":"HOOK_RUN_STATE_FAILED","exitCode":124,"triggeredBy":"create","ranIn":"session"}]}}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"failed (exit 124)", "did not finish within 2 hours", "background"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

// run-hook refuses On create client-side, in the backend's words, without
// a request; the other hooks are queued.
func TestRunHookCmd_OnCreateRefused(t *testing.T) {
	calls := 0
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/workspaces/ws-1/sessions/"+uuidSession+"/hook-runs" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"run":{"id":"r-2","hook":"on_restore","attempt":2,"state":"HOOK_RUN_STATE_PENDING","triggeredBy":"run_again"}}`)
	}))
	defer srv.Close()

	_, _, err := run(t, newRunHookCmd(), srv.URL, "ws-1", []string{uuidSession, "--hook", "on_create"}, output.Human)
	if err == nil {
		t.Fatal("expected an error for --hook on_create")
	}
	if want := "On create cannot be run again on this machine. Use Rebuild to run it on a fresh one."; !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "session rebuild") {
		t.Errorf("error = %q, want the backend's words and the rebuild command", err)
	}
	if calls != 0 {
		t.Errorf("backend was called %d times for a refused hook", calls)
	}

	stdout, _, err := run(t, newRunHookCmd(), srv.URL, "ws-1", []string{uuidSession, "--hook", "on_restore"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["hook"] != "on_restore" {
		t.Errorf("body = %v, want hook on_restore", gotBody)
	}
	if !strings.Contains(stdout, "On restore #2 queued") {
		t.Errorf("stdout missing the queued run:\n%s", stdout)
	}
}

// exec dials the machine itself, so the backend's access gate during On
// create is applied client-side, with the backend's message.
func TestExecCmd_RefusedWhileOnCreateRuns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_RUNNING",
			"sshAddress":"ssh vagrant@example.invalid -p 22","sshPassword":"pw","sshConnectionOpen":true,
			"lifecycle":{"mode":"hooks","phase":"LIFECYCLE_PHASE_SETTING_UP",
				"runs":[{"id":"r-1","hook":"on_create","attempt":1,"state":"HOOK_RUN_STATE_RUNNING","triggeredBy":"create","ranIn":"session"}]}}}`)
	}))
	defer srv.Close()

	_, _, err := run(t, newExecCmd(), srv.URL, "ws-1", []string{uuidSession, "--", "true"}, output.Human)
	if err == nil {
		t.Fatal("expected exec to be refused while On create runs")
	}
	if want := "Session is setting up: On create is still running. Access opens when it finishes."; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want %q", err, want)
	}
}
