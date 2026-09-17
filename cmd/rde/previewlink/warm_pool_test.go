package previewlink

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

// --warm-pool sends warmPoolId and no device: the pool's device applies, and
// --device-platform stops being required.
func TestCreateCmd_WarmPool(t *testing.T) {
	var got map[string]any
	srv := mintServer(t, &got)
	defer srv.Close()

	stdout, _, err := run(t, newCreateCmd(), srv.URL, "ws-1",
		[]string{"--warm-pool", uuidPool, "--artifact-url", "https://example.com/app.apk", "--ttl", "4h"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got["warmPoolId"] != uuidPool {
		t.Errorf("warmPoolId = %v, want %s", got["warmPoolId"], uuidPool)
	}
	for _, k := range []string{"deviceSpec", "stackId", "machineType"} {
		if _, ok := got[k]; ok {
			t.Errorf("%s must not be sent with --warm-pool: %v", k, got)
		}
	}
	if artifact, _ := got["artifact"].(map[string]any); artifact["url"] != "https://example.com/app.apk" {
		t.Errorf("artifact = %v", got["artifact"])
	}
	if got["ttlSeconds"] != float64(14400) {
		t.Errorf("ttlSeconds = %v, want 14400", got["ttlSeconds"])
	}
	if !strings.Contains(stdout, "link-1") {
		t.Errorf("stdout missing the minted link:\n%s", stdout)
	}
}

// A pool NAME resolves through the visible listing before minting.
func TestCreateCmd_WarmPoolResolvesName(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/warm-pools":
			_, _ = io.WriteString(w, `{"warmPools":[{"id":"p-9","name":"ci-devices","ownerType":"workspace"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces/ws-1/preview-links":
			_ = json.NewDecoder(r.Body).Decode(&got)
			_, _ = io.WriteString(w, `{"token":"payload.sig","url":"https://example.com/v/payload.sig","jti":"link-1","expiresAt":"2026-09-16T10:00:00Z"}`)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	if _, _, err := run(t, newCreateCmd(), srv.URL, "ws-1", []string{"--warm-pool", "ci-devices", "--artifact-url", "https://x/a.apk"}, output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got["warmPoolId"] != "p-9" {
		t.Errorf("warmPoolId = %v, want the resolved id p-9", got["warmPoolId"])
	}
}

// The pool fixes the device and the machine: every flag that would
// re-specify them is rejected locally, naming the flag; and without a pool
// the platform is still required.
func TestCreateCmd_WarmPoolRejectsDeviceAndMachineFlags(t *testing.T) {
	cases := map[string][]string{
		"device platform":     {"--warm-pool", uuidPool, "--artifact-url", "https://x/a.apk", "--device-platform", "android"},
		"device model":        {"--warm-pool", uuidPool, "--artifact-url", "https://x/a.apk", "--device-model", "pixel_7"},
		"device os version":   {"--warm-pool", uuidPool, "--artifact-url", "https://x/a.zip", "--device-os-version", "18.2"},
		"device system image": {"--warm-pool", uuidPool, "--artifact-url", "https://x/a.apk", "--device-system-image", "system-images;android-34;google_apis;x86_64"},
		"stack":               {"--warm-pool", uuidPool, "--artifact-url", "https://x/a.apk", "--stack", "osx-27-edge"},
		"machine type":        {"--warm-pool", uuidPool, "--artifact-url", "https://x/a.apk", "--machine-type", "g2.mac"},
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
			if !strings.Contains(err.Error(), "--warm-pool") || !strings.Contains(err.Error(), args[4]) {
				t.Errorf("error %q should name --warm-pool and %s", err, args[4])
			}
		})
	}

	c := newCreateCmd()
	c.SilenceUsage, c.SilenceErrors = true, true
	_, _, err := run(t, c, "http://unused", "ws-1", []string{"--artifact-url", "https://x/a.apk"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--device-platform must be ios or android") {
		t.Errorf("error = %v, want the platform to stay required without --warm-pool", err)
	}
}

// The artifact stays required with a pool: a link exists to show a build.
func TestCreateCmd_WarmPoolStillNeedsArtifact(t *testing.T) {
	c := newCreateCmd()
	c.SilenceUsage, c.SilenceErrors = true, true
	_, _, err := run(t, c, "http://unused", "ws-1", []string{"--warm-pool", uuidPool}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--artifact-url or --artifact-url-stdin is required") {
		t.Errorf("error = %v, want the artifact to be required", err)
	}
}
