package session

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-cli/internal/output"
)

// uuidPool is a UUID-shaped warm pool arg so ResolveWarmPoolID
// short-circuits without an extra ListWarmPools call.
const uuidPool = "33333333-4444-4444-8444-555555555555"

// --warm-pool sends warmPoolId and nothing that would shape the VM; the
// per-session fields (description, labels, auto-terminate) still apply.
func TestCreateCmd_WarmPoolClaim(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/workspaces/ws-1/sessions" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"session":{"id":"s-new","name":"dev","status":"SESSION_STATUS_RUNNING","warmPoolId":"`+uuidPool+`","warmState":"claimed"}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newCreateCmd(), srv.URL, "ws-1",
		[]string{"dev", "--warm-pool", uuidPool, "--description", "from pool", "--label", "team=mobile", "--auto-terminate-minutes", "90"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["warmPoolId"] != uuidPool || gotBody["name"] != "dev" || gotBody["description"] != "from pool" || gotBody["autoTerminateMinutes"] != float64(90) {
		t.Errorf("unexpected create body: %v", gotBody)
	}
	if labels, _ := gotBody["labels"].(map[string]any); labels["team"] != "mobile" {
		t.Errorf("labels not forwarded: %v", gotBody["labels"])
	}
	for _, k := range []string{"templateId", "stackId", "machineType", "sessionInputs", "enabledFeatureFlagNames", "cluster", "aiPrompt", "deviceSpec", "noDevice", "mapSavedToSessionInputs", "ownerType"} {
		if _, ok := gotBody[k]; ok {
			t.Errorf("%s must not be sent with --warm-pool: %v", k, gotBody)
		}
	}
	for _, want := range []string{"Session created", "s-new", "Warm pool:", uuidPool, "Warm state:", "claimed", "pre-booted"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

// --owner may accompany --warm-pool (it must match the pool's owner, which
// the backend checks), and an artifact applies to a device pool's session.
func TestCreateCmd_WarmPoolAllowsOwnerAndArtifact(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"session":{"id":"s-new","name":"ci","status":"SESSION_STATUS_RUNNING","warmState":"cold"}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newCreateCmd(), srv.URL, "ws-1",
		[]string{"ci", "--warm-pool", uuidPool, "--owner", "workspace", "--artifact-url", "https://example.com/app.apk", "--artifact-name", "Demo"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["ownerType"] != "workspace" {
		t.Errorf("ownerType = %v, want workspace", gotBody["ownerType"])
	}
	if artifact, _ := gotBody["artifact"].(map[string]any); artifact["url"] != "https://example.com/app.apk" || artifact["appName"] != "Demo" {
		t.Errorf("artifact = %v", gotBody["artifact"])
	}
	if _, ok := gotBody["deviceSpec"]; ok {
		t.Errorf("deviceSpec must not be sent with --warm-pool: %v", gotBody)
	}
	if !strings.Contains(stdout, "cold") || !strings.Contains(stdout, "no warm session was available") {
		t.Errorf("stdout missing the cold warm state:\n%s", stdout)
	}
}

// A pool NAME resolves through the visible listing before the claim.
func TestCreateCmd_WarmPoolResolvesName(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/warm-pools":
			_, _ = io.WriteString(w, `{"warmPools":[{"id":"p-9","name":"ios-devs"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces/ws-1/sessions":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_, _ = io.WriteString(w, `{"session":{"id":"s-new","name":"dev","status":"SESSION_STATUS_RUNNING"}}`)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	if _, _, err := run(t, newCreateCmd(), srv.URL, "ws-1", []string{"dev", "--warm-pool", "ios-devs"}, output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["warmPoolId"] != "p-9" {
		t.Errorf("warmPoolId = %v, want the resolved id p-9", gotBody["warmPoolId"])
	}
}

// Every flag that would shape the VM is rejected locally with --warm-pool,
// naming the offending flag, before any round trip.
func TestCreateCmd_WarmPoolRejectsConfiguration(t *testing.T) {
	cases := map[string][]string{
		"template":            {"dev", "--warm-pool", uuidPool, "--template", uuidTemplate},
		"stack":               {"dev", "--warm-pool", uuidPool, "--stack", "osx-27-edge"},
		"machine type":        {"dev", "--warm-pool", uuidPool, "--machine-type", "g2.mac"},
		"input":               {"dev", "--warm-pool", uuidPool, "--input", "k=v"},
		"secret input":        {"dev", "--warm-pool", uuidPool, "--secret-input", "k=v"},
		"saved input":         {"dev", "--warm-pool", uuidPool, "--saved-input", "k=" + uuidSession},
		"map saved inputs":    {"dev", "--warm-pool", uuidPool, "--map-saved-inputs"},
		"feature flag":        {"dev", "--warm-pool", uuidPool, "--feature-flag", "beta"},
		"cluster":             {"dev", "--warm-pool", uuidPool, "--cluster", "c1"},
		"ai prompt":           {"dev", "--warm-pool", uuidPool, "--ai-prompt", "hi"},
		"device platform":     {"dev", "--warm-pool", uuidPool, "--device-platform", "ios"},
		"device model":        {"dev", "--warm-pool", uuidPool, "--device-model", "iPhone 16"},
		"device os version":   {"dev", "--warm-pool", uuidPool, "--device-os-version", "18.2"},
		"device system image": {"dev", "--warm-pool", uuidPool, "--device-system-image", "system-images;android-34;google_apis;x86_64"},
		"no device":           {"dev", "--warm-pool", uuidPool, "--no-device"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("backend must not be called for an invalid request")
			}))
			defer srv.Close()
			c := newCreateCmd()
			c.SilenceUsage, c.SilenceErrors = true, true
			_, _, err := run(t, c, srv.URL, "ws-1", args, output.Human)
			if err == nil {
				t.Fatal("expected an error, got none")
			}
			if !strings.Contains(err.Error(), "--warm-pool") || !strings.Contains(err.Error(), args[3]) {
				t.Errorf("error %q should name --warm-pool and the conflicting flag %s", err, args[3])
			}
		})
	}
}

// The "provide --template ..." guard must accept --warm-pool alone and
// mention it when nothing was given.
func TestCreateCmd_WarmPoolSatisfiesConfigurationGuard(t *testing.T) {
	_, _, err := run(t, newCreateCmd(), "http://unused", "ws-1", []string{"dev"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--warm-pool") {
		t.Errorf("error = %v, want it to offer --warm-pool", err)
	}
}

// session view shows the warm pool and state only when the session has one;
// JSON carries them additively.
func TestViewCmd_ShowsWarmState(t *testing.T) {
	body := `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_RUNNING","warmPoolId":"p-1","warmState":"claimed"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"Warm pool:", "p-1", "Warm state:", "claimed"} {
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
	if got["warm_pool_id"] != "p-1" || got["warm_state"] != "claimed" {
		t.Errorf("unexpected JSON: %v", got)
	}

	// No pool: no warm lines, no warm keys.
	body = `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_RUNNING"}}`
	stdout, _, err = run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute (plain): %v", err)
	}
	if strings.Contains(stdout, "Warm") {
		t.Errorf("stdout must not mention warm pools for a pool-less session:\n%s", stdout)
	}
	stdout, _, err = run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.JSON)
	if err != nil {
		t.Fatalf("Execute (plain json): %v", err)
	}
	if strings.Contains(stdout, "warm_") {
		t.Errorf("JSON must not gain warm_* keys for a pool-less session:\n%s", stdout)
	}
}
