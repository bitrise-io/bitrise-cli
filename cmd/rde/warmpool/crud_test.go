package warmpool

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-cli/internal/output"
)

// mutationServer answers one request on path with body and records the JSON
// it received.
func mutationServer(t *testing.T, method, path, body string, got *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method || r.URL.Path != path {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		*got = nil
		_ = json.NewDecoder(r.Body).Decode(got)
		_, _ = io.WriteString(w, body)
	}))
}

func TestCreateCmd_HappyPath(t *testing.T) {
	var gotBody map[string]any
	srv := mutationServer(t, http.MethodPost, "/v1/workspaces/ws-1/warm-pools",
		`{"warmPool":{"id":"p-new","name":"ios-devs","templateName":"iOS Dev","ownerType":"workspace","desiredCount":2,"status":{}}}`, &gotBody)
	defer srv.Close()

	stdout, _, err := run(t, newCreateCmd(), srv.URL, "ws-1", []string{
		"ios-devs", "--template", uuidTemplate, "--count", "2", "--owner", "workspace",
		"--input", "REPO=my-app", "--secret-input", "GITHUB_TOKEN=ghp_x",
		"--feature-flag", "beta", "--machine-type", "g2.mac", "--cluster", "c1",
	}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["name"] != "ios-devs" || gotBody["templateId"] != uuidTemplate || gotBody["ownerType"] != "workspace" || gotBody["desiredCount"] != float64(2) {
		t.Errorf("unexpected create body: %v", gotBody)
	}
	if gotBody["machineType"] != "g2.mac" || gotBody["cluster"] != "c1" {
		t.Errorf("overrides not forwarded: %v", gotBody)
	}
	if _, ok := gotBody["stackId"]; ok {
		t.Errorf("stackId must be omitted when unset: %v", gotBody)
	}
	inputs, _ := gotBody["sessionInputs"].([]any)
	if len(inputs) != 2 {
		t.Fatalf("sessionInputs = %v", gotBody["sessionInputs"])
	}
	if secret, _ := inputs[1].(map[string]any); secret["key"] != "GITHUB_TOKEN" || secret["isSecret"] != true {
		t.Errorf("secret input wrong: %v", inputs[1])
	}
	if flags, _ := gotBody["enabledFeatureFlagNames"].([]any); len(flags) != 1 || flags[0] != "beta" {
		t.Errorf("feature flags wrong: %v", gotBody["enabledFeatureFlagNames"])
	}
	if !strings.Contains(stdout, "p-new") || !strings.Contains(stdout, "Desired count:") {
		t.Errorf("stdout missing the created pool:\n%s", stdout)
	}
}

// The default is an inert preset: nothing booted, no owner sent (the backend
// picks the token's default), no saved-input mapping.
func TestCreateCmd_DefaultsAreMinimal(t *testing.T) {
	var gotBody map[string]any
	srv := mutationServer(t, http.MethodPost, "/v1/workspaces/ws-1/warm-pools", `{"warmPool":{"id":"p-new","name":"preset"}}`, &gotBody)
	defer srv.Close()

	if _, _, err := run(t, newCreateCmd(), srv.URL, "ws-1", []string{"preset", "--template", uuidTemplate}, output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, k := range []string{"desiredCount", "ownerType", "sessionInputs", "mapSavedToSessionInputs", "enabledFeatureFlagNames"} {
		if _, ok := gotBody[k]; ok {
			t.Errorf("%s should be omitted by default, body = %v", k, gotBody)
		}
	}
}

// A user pool may reference saved inputs and ask for auto-mapping.
func TestCreateCmd_SavedInputsOnUserPool(t *testing.T) {
	var gotBody map[string]any
	srv := mutationServer(t, http.MethodPost, "/v1/workspaces/ws-1/warm-pools", `{"warmPool":{"id":"p-new","name":"mine"}}`, &gotBody)
	defer srv.Close()

	if _, _, err := run(t, newCreateCmd(), srv.URL, "ws-1",
		[]string{"mine", "--template", uuidTemplate, "--saved-input", "gh-token=si-1", "--map-saved-inputs"}, output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	inputs, _ := gotBody["sessionInputs"].([]any)
	if len(inputs) != 1 {
		t.Fatalf("sessionInputs = %v", gotBody["sessionInputs"])
	}
	if in, _ := inputs[0].(map[string]any); in["savedInputId"] != "si-1" {
		t.Errorf("saved input wrong: %v", inputs[0])
	}
	if gotBody["mapSavedToSessionInputs"] != true {
		t.Errorf("mapSavedToSessionInputs = %v, want true", gotBody["mapSavedToSessionInputs"])
	}
}

// A template NAME resolves through ListTemplates; the create body carries
// the ID.
func TestCreateCmd_ResolvesTemplateName(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/templates":
			_, _ = io.WriteString(w, `{"templates":[{"id":"t-9","name":"iOS Dev"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces/ws-1/warm-pools":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_, _ = io.WriteString(w, `{"warmPool":{"id":"p-new","name":"ios-devs"}}`)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	if _, _, err := run(t, newCreateCmd(), srv.URL, "ws-1", []string{"ios-devs", "--template", "iOS Dev"}, output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["templateId"] != "t-9" {
		t.Errorf("templateId = %v, want the resolved id t-9", gotBody["templateId"])
	}
}

// Everything the backend is certain to reject fails before the round trip.
func TestCreateCmd_RejectsBadFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no template", []string{"ios-devs"}, "--template is required"},
		{"no name", nil, "NAME"},
		{"negative count", []string{"ios-devs", "--template", uuidTemplate, "--count", "-1"}, "--count must not be negative"},
		{"unknown owner", []string{"ios-devs", "--template", uuidTemplate, "--owner", "agent"}, "--owner must be"},
		{"saved input on workspace pool", []string{"ios-devs", "--template", uuidTemplate, "--owner", "workspace", "--saved-input", "k=si-1"}, "saved inputs are personal"},
		{"map saved on workspace pool", []string{"ios-devs", "--template", uuidTemplate, "--owner", "workspace", "--map-saved-inputs"}, "saved inputs are personal"},
		{"malformed input", []string{"ios-devs", "--template", uuidTemplate, "--input", "novalue"}, "expected key=value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("backend must not be called for an invalid request")
			}))
			defer srv.Close()
			c := newCreateCmd()
			c.SilenceUsage, c.SilenceErrors = true, true
			_, _, err := run(t, c, srv.URL, "ws-1", tc.args, output.Human)
			if err == nil {
				t.Fatalf("expected an error, got none")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

// Scalars go out only when given; a list flag replaces the list and carries
// its switch; untouched groups leave no trace.
func TestUpdateCmd_SendsOnlyWhatChanged(t *testing.T) {
	var gotBody map[string]any
	srv := mutationServer(t, http.MethodPatch, "/v1/workspaces/ws-1/warm-pools/"+uuidPool, `{"warmPool":{"id":"p-1","name":"renamed"}}`, &gotBody)
	defer srv.Close()

	if _, _, err := run(t, newUpdateCmd(), srv.URL, "ws-1",
		[]string{uuidPool, "--name", "renamed", "--count", "0", "--secret-input", "GITHUB_TOKEN=ghp_y", "--stack", ""}, output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["name"] != "renamed" {
		t.Errorf("name = %v, want renamed", gotBody["name"])
	}
	// --count 0 is a real request (drain the pool) and must be sent.
	if v, ok := gotBody["desiredCount"]; !ok || v != float64(0) {
		t.Errorf("desiredCount = %v (present=%v), want 0 present", v, ok)
	}
	// --stack "" clears the override and must be sent as an empty string.
	if v, ok := gotBody["stackId"]; !ok || v != "" {
		t.Errorf("stackId = %v (present=%v), want \"\" present", v, ok)
	}
	if gotBody["updateSessionInputs"] != true {
		t.Errorf("updateSessionInputs = %v, want true (body=%v)", gotBody["updateSessionInputs"], gotBody)
	}
	inputs, _ := gotBody["sessionInputs"].([]any)
	if len(inputs) != 1 {
		t.Errorf("sessionInputs = %v", gotBody["sessionInputs"])
	}
	for _, k := range []string{"updateEnabledFeatureFlagNames", "enabledFeatureFlagNames", "machineType", "cluster", "updateDeviceSpec"} {
		if _, ok := gotBody[k]; ok {
			t.Errorf("%s should be absent, body=%v", k, gotBody)
		}
	}
}

// --clear-inputs / --clear-feature-flags are the switch alone.
func TestUpdateCmd_ClearLists(t *testing.T) {
	var gotBody map[string]any
	srv := mutationServer(t, http.MethodPatch, "/v1/workspaces/ws-1/warm-pools/"+uuidPool, `{"warmPool":{"id":"p-1","name":"ios-devs"}}`, &gotBody)
	defer srv.Close()

	if _, _, err := run(t, newUpdateCmd(), srv.URL, "ws-1", []string{uuidPool, "--clear-inputs", "--clear-feature-flags"}, output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["updateSessionInputs"] != true || gotBody["updateEnabledFeatureFlagNames"] != true {
		t.Errorf("switches missing, body=%v", gotBody)
	}
	for _, k := range []string{"sessionInputs", "enabledFeatureFlagNames", "name", "desiredCount"} {
		if _, ok := gotBody[k]; ok {
			t.Errorf("%s should be absent, body=%v", k, gotBody)
		}
	}
}

func TestUpdateCmd_RejectsBadFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"nothing to update", []string{uuidPool}, "nothing to update"},
		{"clear and set inputs", []string{uuidPool, "--clear-inputs", "--input", "k=v"}, "--clear-inputs cannot be combined"},
		{"clear and set flags", []string{uuidPool, "--clear-feature-flags", "--feature-flag", "beta"}, "--clear-feature-flags cannot be combined"},
		{"negative count", []string{uuidPool, "--count", "-2"}, "--count must not be negative"},
		{"empty name", []string{uuidPool, "--name", ""}, "--name must not be empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("backend must not be called for an invalid request")
			}))
			defer srv.Close()
			c := newUpdateCmd()
			c.SilenceUsage, c.SilenceErrors = true, true
			_, _, err := run(t, c, srv.URL, "ws-1", tc.args, output.Human)
			if err == nil {
				t.Fatalf("expected an error, got none")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

func TestUpdateCmd_RequiresArg(t *testing.T) {
	_, _, err := run(t, newUpdateCmd(), "http://unused", "ws-1", []string{"--count", "1"}, output.Human)
	if err == nil {
		t.Fatal("expected error when WARM_POOL_ID is missing")
	}
}

// set-count is a PATCH of the count alone; the confirmation goes to stderr
// and, with --output json, the pool goes to stdout.
func TestSetCountCmd_HappyPath(t *testing.T) {
	var gotBody map[string]any
	srv := mutationServer(t, http.MethodPatch, "/v1/workspaces/ws-1/warm-pools/"+uuidPool, `{"warmPool":{"id":"p-1","name":"ios-devs","desiredCount":3}}`, &gotBody)
	defer srv.Close()

	stdout, stderr, err := run(t, newSetCountCmd(), srv.URL, "ws-1", []string{uuidPool, "3"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["desiredCount"] != float64(3) || len(gotBody) != 1 {
		t.Errorf("body = %v, want only desiredCount 3", gotBody)
	}
	if stdout != "" {
		t.Errorf("stdout should be empty in human mode, got: %q", stdout)
	}
	if !strings.Contains(stderr, "ios-devs now keeps 3 warm session(s)") {
		t.Errorf("stderr missing confirmation: %q", stderr)
	}

	// 0 drains the pool and is a legal count.
	if _, _, err := run(t, newSetCountCmd(), srv.URL, "ws-1", []string{uuidPool, "0"}, output.Human); err != nil {
		t.Fatalf("Execute (0): %v", err)
	}
	if v, ok := gotBody["desiredCount"]; !ok || v != float64(0) {
		t.Errorf("desiredCount = %v (present=%v), want 0 present", v, ok)
	}

	stdout, _, err = run(t, newSetCountCmd(), srv.URL, "ws-1", []string{uuidPool, "3"}, output.JSON)
	if err != nil {
		t.Fatalf("Execute (json): %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if got["desired_count"] != float64(3) {
		t.Errorf("unexpected JSON: %v", got)
	}
}

func TestSetCountCmd_RejectsBadCount(t *testing.T) {
	for _, count := range []string{"-1", "two", ""} {
		c := newSetCountCmd()
		c.SilenceUsage, c.SilenceErrors = true, true
		// "--" keeps a negative number from being read as a flag.
		_, _, err := run(t, c, "http://unused", "ws-1", []string{uuidPool, "--", count}, output.Human)
		if err == nil || !strings.Contains(err.Error(), "COUNT must be a non-negative integer") {
			t.Errorf("count %q: error = %v, want a COUNT error", count, err)
		}
	}
	if _, _, err := run(t, newSetCountCmd(), "http://unused", "ws-1", []string{uuidPool}, output.Human); err == nil {
		t.Error("expected error when COUNT is missing")
	}
}

func TestDeleteCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/workspaces/ws-1/warm-pools/"+uuidPool {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	stdout, stderr, err := run(t, newDeleteCmd(), srv.URL, "ws-1", []string{uuidPool, "--yes"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// Confirmation goes to stderr, never stdout.
	if !strings.Contains(stderr, "Deleted warm pool "+uuidPool) {
		t.Errorf("stderr missing confirmation: %q", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout should be empty for delete, got: %q", stdout)
	}
}

// Without --yes the prompt is read from stdin; anything but y/yes aborts
// before the round trip.
func TestDeleteCmd_PromptAborts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("backend must not be called when the prompt is declined")
	}))
	defer srv.Close()

	c := newDeleteCmd()
	c.SetIn(strings.NewReader("n\n"))
	_, stderr, err := run(t, c, srv.URL, "ws-1", []string{uuidPool}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Errorf("error = %v, want aborted", err)
	}
	if !strings.Contains(stderr, "Proceed? [y/N]") {
		t.Errorf("stderr missing the prompt: %q", stderr)
	}
}

func TestDeleteCmd_PromptAccepts(t *testing.T) {
	deleted := false
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		deleted = r.Method == http.MethodDelete
	}))
	defer srv.Close()

	c := newDeleteCmd()
	c.SetIn(strings.NewReader("y\n"))
	if _, _, err := run(t, c, srv.URL, "ws-1", []string{uuidPool}, output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !deleted {
		t.Error("expected the DELETE to go out after confirming")
	}
}

// TestDeleteCmd_ResolvesName covers name → ID resolution: a non-UUID arg is
// looked up via ListWarmPools and the resolved ID is what gets deleted.
func TestDeleteCmd_ResolvesName(t *testing.T) {
	var deletedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/warm-pools":
			_, _ = io.WriteString(w, `{"warmPools":[{"id":"p-9","name":"ios-devs"},{"id":"p-7","name":"other"}]}`)
		case r.Method == http.MethodDelete:
			deletedPath = r.URL.Path
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	_, stderr, err := run(t, newDeleteCmd(), srv.URL, "ws-1", []string{"ios-devs", "--yes"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if deletedPath != "/v1/workspaces/ws-1/warm-pools/p-9" {
		t.Errorf("deleted path = %q, want the resolved id p-9", deletedPath)
	}
	if !strings.Contains(stderr, "Deleted warm pool p-9") {
		t.Errorf("stderr should confirm deletion of the resolved id: %q", stderr)
	}
}

func TestDeleteCmd_RequiresArg(t *testing.T) {
	_, _, err := run(t, newDeleteCmd(), "http://unused", "ws-1", nil, output.Human)
	if err == nil {
		t.Fatal("expected error when WARM_POOL_ID is missing")
	}
}
