package previewlink

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

// mintServer answers one mint request and records the body it received.
func mintServer(t *testing.T, got *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/workspaces/ws-1/preview-links" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(got)
		_, _ = io.WriteString(w, `{"token":"payload.sig",
			"url":"https://app.bitrise.io/dev-environments/ws-1/device-preview/payload.sig",
			"jti":"link-1","expiresAt":"2026-09-16T10:00:00Z"}`)
	}))
}

// The human output must carry the shareable URL: nothing is stored
// server-side, so this is the only place the caller will ever see it.
func TestCreateCmd_HappyPath(t *testing.T) {
	var got map[string]any
	srv := mintServer(t, &got)
	defer srv.Close()

	stdout, _, err := run(t, newCreateCmd(), srv.URL, "ws-1",
		[]string{"--device-platform", "android", "--artifact-url", "https://example.com/app.apk", "--artifact-name", "Demo"},
		output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	spec, _ := got["deviceSpec"].(map[string]any)
	if spec["platform"] != "android" {
		t.Errorf("deviceSpec = %v, want platform android", got["deviceSpec"])
	}
	artifact, _ := got["artifact"].(map[string]any)
	if artifact["url"] != "https://example.com/app.apk" || artifact["appName"] != "Demo" {
		t.Errorf("artifact = %v", got["artifact"])
	}
	// Unset knobs must not reach the wire: 0 means "backend default" on the
	// TTL, but an empty stack/machine type would be a different request.
	for _, key := range []string{"ttlSeconds", "sessionAutoTerminateMinutes", "stackId", "machineType"} {
		if v, present := got[key]; present {
			t.Errorf("%s = %v present in body, want absent", key, v)
		}
	}
	for _, want := range []string{"device-preview/payload.sig", "link-1", "2026-09-16T10:00:00Z", "cannot be revoked"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

// --ttl is a duration for humans but seconds on the wire.
func TestCreateCmd_ForwardsOptionalKnobs(t *testing.T) {
	var got map[string]any
	srv := mintServer(t, &got)
	defer srv.Close()

	if _, _, err := run(t, newCreateCmd(), srv.URL, "ws-1", []string{
		"--device-platform", "ios",
		"--artifact-url", "https://example.com/App.zip",
		"--device-model", "iPhone 16",
		"--device-os-version", "18.2",
		"--ttl", "4h",
		"--auto-terminate-minutes", "15",
		"--stack", "osx-27-edge",
		"--machine-type", "g2.mac.m2pro.4c",
	}, output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got["ttlSeconds"] != float64(14400) {
		t.Errorf("ttlSeconds = %v, want 14400", got["ttlSeconds"])
	}
	if got["sessionAutoTerminateMinutes"] != float64(15) {
		t.Errorf("sessionAutoTerminateMinutes = %v, want 15", got["sessionAutoTerminateMinutes"])
	}
	if got["stackId"] != "osx-27-edge" || got["machineType"] != "g2.mac.m2pro.4c" {
		t.Errorf("machine overrides = %v / %v", got["stackId"], got["machineType"])
	}
	spec, _ := got["deviceSpec"].(map[string]any)
	if spec["deviceModel"] != "iPhone 16" || spec["osVersion"] != "18.2" {
		t.Errorf("deviceSpec = %v", got["deviceSpec"])
	}
}

// The JSON shape is the stable contract a CI step parses.
func TestCreateCmd_JSONOutput(t *testing.T) {
	var got map[string]any
	srv := mintServer(t, &got)
	defer srv.Close()

	stdout, _, err := run(t, newCreateCmd(), srv.URL, "ws-1",
		[]string{"--device-platform", "ios", "--artifact-url", "https://example.com/App.zip"}, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var link struct {
		URL       string `json:"url"`
		Token     string `json:"token"`
		JTI       string `json:"jti"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal([]byte(stdout), &link); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if link.URL == "" || link.Token != "payload.sig" || link.JTI != "link-1" {
		t.Errorf("unexpected JSON: %+v", link)
	}
	if !strings.HasPrefix(link.ExpiresAt, "2026-09-16T10:00:00") {
		t.Errorf("expires_at = %q", link.ExpiresAt)
	}
}

// Everything the backend is certain to reject fails before the round trip, so
// a CI step gets the reason instead of a 400 it has to interpret.
func TestCreateCmd_RejectsBadFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"bad platform", []string{"--device-platform", "windows", "--artifact-url", "https://x/a.apk"}, "--device-platform must be ios or android"},
		{"no artifact", []string{"--device-platform", "ios"}, "--artifact-url or --artifact-url-stdin is required"},
		{"system image on ios", []string{"--device-platform", "ios", "--artifact-url", "https://x/a.zip", "--device-system-image", "system-images;android-34;google_apis;x86_64"}, "applies to Android only"},
		{"ttl over cap", []string{"--device-platform", "ios", "--artifact-url", "https://x/a.zip", "--ttl", "100h"}, "--ttl must be at most 72h"},
		{"negative ttl", []string{"--device-platform", "ios", "--artifact-url", "https://x/a.zip", "--ttl", "-1h"}, "--ttl must not be negative"},
		{"idle window under the floor", []string{"--device-platform", "ios", "--artifact-url", "https://x/a.zip", "--auto-terminate-minutes", "5"}, "--auto-terminate-minutes must be at least 10"},
		{"negative idle window", []string{"--device-platform", "ios", "--artifact-url", "https://x/a.zip", "--auto-terminate-minutes", "-1"}, "--auto-terminate-minutes must not be negative"},
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
