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
			{"id":"t-1","name":"Linux Dev","stackId":"linux-ubuntu-24.04","machineType":"standard","createdByEmail":"a@b.io"}
		]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", nil, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"Linux Dev", "linux-ubuntu-24.04", "standard", "a@b.io", "t-1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestListCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"templates":[
			{"id":"t-1","name":"Linux Dev","stackId":"linux-ubuntu-24.04","machineType":"standard"}
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
		if r.URL.Path != "/v1/workspaces/ws-1/templates/"+uuidTemplateID {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"template":{
			"id":"t-1","name":"Linux Dev","stackId":"linux-ubuntu-24.04","machineType":"standard",
			"sessionInputs":[{"key":"repo","required":true,"description":"Repo to clone"}],
			"templateVariables":[{"key":"TOKEN","isSecret":true}]
		}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidTemplateID}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"Linux Dev", "t-1", "linux-ubuntu-24.04", "repo", "(required)", "TOKEN", "(secret)"} {
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

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidTemplateID}, output.JSON)
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

// TestViewCmd_ShowsDeclaredDevice: a template with a device_spec renders a
// "Device:" line in the same platform · model · version form 'session view'
// uses, and carries device_spec in JSON output for the round-trip workflow.
func TestViewCmd_ShowsDeclaredDevice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"template":{"id":"t-1","name":"iOS Dev","stackId":"osx-xcode-16.0.x-edge","machineType":"g2.mac",
			"deviceSpec":{"platform":"ios","deviceModel":"iPhone 16","osVersion":"18.2"}}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidTemplateID}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "Device:") || !strings.Contains(stdout, "iOS simulator · iPhone 16 · 18.2") {
		t.Errorf("stdout missing the declared device:\n%s", stdout)
	}

	stdout, _, err = run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidTemplateID}, output.JSON)
	if err != nil {
		t.Fatalf("Execute (json): %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	spec, _ := got["device_spec"].(map[string]any)
	if spec["platform"] != "ios" || spec["device_model"] != "iPhone 16" {
		t.Errorf("unexpected device_spec in JSON: %v", got["device_spec"])
	}
}

func TestViewCmd_ShowsLifecycleHooks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"template":{"id":"t-1","name":"Hooks Dev","stackId":"linux-ubuntu-24.04","machineType":"standard",
			"lifecycleMode":"hooks","hooks":{"onCreate":"git clone x\nnpm ci","onRestore":"git pull"},
			"startupScript":"# Generated from the template's lifecycle hooks\n...",
			"sessionInputs":[{"key":"GH_TOKEN","required":true,"hook":"on_claim"},{"key":"REPO","hook":"on_create","defaultValue":"app"}]}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidTemplateID}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"Lifecycle:", "hooks", "Lifecycle hooks", "On create", "2 lines", "On claim", "(not added)", "On restore", "1 line", "GH_TOKEN (required, on claim)"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}

	stdout, _, err = run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidTemplateID}, output.JSON)
	if err != nil {
		t.Fatalf("Execute (json): %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if got["lifecycle_mode"] != "hooks" {
		t.Errorf("lifecycle_mode = %v, want hooks", got["lifecycle_mode"])
	}
	hooks, _ := got["hooks"].(map[string]any)
	if hooks["on_create"] != "git clone x\nnpm ci" || hooks["on_restore"] != "git pull" {
		t.Errorf("unexpected hooks in JSON: %v", got["hooks"])
	}
	if _, ok := hooks["on_claim"]; ok {
		t.Errorf("an unset hook must be omitted from JSON: %v", hooks)
	}
	// The generated legacy startup is derived and refused on writes: it
	// must not ride along in the round-trip JSON.
	if _, ok := got["startup_script"]; ok {
		t.Errorf("startup_script must be omitted on a hooks template: %v", got["startup_script"])
	}
	inputs, _ := got["session_inputs"].([]any)
	if in, _ := inputs[0].(map[string]any); in["hook"] != "on_claim" {
		t.Errorf("session_inputs[0].hook = %v, want on_claim", in["hook"])
	}
}

// On a hooks template every variable and input shows its scope (a missing
// hook is shared) and a protected variable says so instead of "secret".
func TestViewCmd_MarksScopesAndProtected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"template":{"id":"t-1","name":"Hooks","lifecycleMode":"hooks","hooks":{"onCreate":"echo"},
			"sessionInputs":[{"key":"GH_TOKEN","required":true,"hook":"on_claim"},{"key":"GH_TOKEN","required":true,"hook":"on_restore"},{"key":"REPO","hook":"shared"},{"key":"LEGACY"}],
			"templateVariables":[{"key":"REGISTRY_TOKEN","isSecret":true,"protected":true,"hook":"on_create"},{"key":"TOKEN","isSecret":true},{"key":"APP","value":"demo","hook":"shared"}]}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidTemplateID}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{
		"GH_TOKEN (required, on claim)", "GH_TOKEN (required, on restore)", "REPO (shared)", "LEGACY (shared)",
		"REGISTRY_TOKEN (protected, on create)", "TOKEN (secret, shared)", "APP (shared)",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "Template variables") {
		t.Errorf("the heading is Variables now:\n%s", stdout)
	}
}

// A legacy template's values are all shared, so no scope is shown.
func TestViewCmd_LegacyTemplateShowsNoScopes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"template":{"id":"t-1","name":"Legacy","lifecycleMode":"legacy","startupScript":"echo",
			"sessionInputs":[{"key":"REPO","required":true,"hook":"shared"}],
			"templateVariables":[{"key":"TOKEN","isSecret":true,"hook":"shared"},{"key":"APP","value":"demo"}]}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidTemplateID}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"REPO (required)\n", "TOKEN (secret)\n", "  APP\n"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "shared") {
		t.Errorf("a legacy template must not tag scopes:\n%s", stdout)
	}
}
