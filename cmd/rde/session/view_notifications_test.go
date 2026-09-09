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

func TestViewCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/ws-1/sessions/"+uuidSession {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"session":{
			"id":"s-1","name":"dev","status":"SESSION_STATUS_RUNNING",
			"templateId":"t-1","sshAddress":"ssh://host:22",
			"templateSnapshot":{"stackId":"osx-xcode-16.0.x-edge","machineType":"g2.mac"}
		}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"dev", "s-1", "running", "ssh://host:22", "osx-xcode-16.0.x-edge", "g2.mac"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestViewCmd_JSONOutput_MapsStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_RUNNING"}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if got["id"] != "s-1" || got["status"] != "running" {
		t.Errorf("unexpected JSON: %v", got)
	}
}

// TestViewCmd_JSONOmitsSSHPassword pins the security contract: the SSH
// password the API returns (consumed internally by `session exec`) must
// never appear in the stable --output json shape. Session.SSHPassword
// carries json:"-".
func TestViewCmd_JSONOmitsSSHPassword(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","sshAddress":"ssh://h:22","sshPassword":"hunter2"}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Contains(stdout, "hunter2") || strings.Contains(stdout, "ssh_password") {
		t.Errorf("SSH password leaked into JSON output:\n%s", stdout)
	}
	// ...but the non-secret SSH address is still part of the contract.
	if !strings.Contains(stdout, "ssh_address") {
		t.Errorf("ssh_address missing from JSON:\n%s", stdout)
	}
}

// TestViewCmd_DeviceIOS: a device session's human output carries a Device
// line with platform, model and iOS version plus the normalized state, the
// device notes, the app install state with its failure reason, and the
// viewer URL.
func TestViewCmd_DeviceIOS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{
			"id":"s-1","name":"ios-check","status":"SESSION_STATUS_RUNNING",
			"device":{
				"spec":{"platform":"ios","deviceModel":"iPhone 16","osVersion":"com.apple.CoreSimulator.SimRuntime.iOS-18-2"},
				"state":"PREVIEW_DEVICE_STATE_READY",
				"installStatus":"PREVIEW_INSTALL_STATUS_FAILED",
				"installReason":"artifact is not a zipped simulator .app",
				"deviceNotes":"warm boot from snapshot",
				"appName":"Demo","buildNumber":"42",
				"viewerUrl":"https://viewer.example.com/d/abc"
			}
		}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{
		"Device:", "iOS simulator · iPhone 16 · com.apple.CoreSimulator.SimRuntime.iOS-18-2 — ready",
		"Device notes:", "warm boot from snapshot",
		"Device app:", "Demo #42 — install failed: artifact is not a zipped simulator .app",
		"Device view:", "https://viewer.example.com/d/abc",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	// The wire enum names must not leak into human output.
	if strings.Contains(stdout, "PREVIEW_") {
		t.Errorf("wire enum leaked into human output:\n%s", stdout)
	}
}

// TestViewCmd_DeviceAndroidSystemImage: Android specs carry no osVersion —
// the system image package identifies the OS, so the Device line falls back
// to it. A booting device with no app shows neither an app nor a viewer line.
func TestViewCmd_DeviceAndroidSystemImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{
			"id":"s-2","name":"android-check","status":"SESSION_STATUS_RUNNING",
			"device":{
				"spec":{"platform":"android","deviceModel":"pixel_7","systemImage":"system-images;android-34;google_apis;x86_64"},
				"state":"PREVIEW_DEVICE_STATE_BOOTING"
			}
		}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := "Android emulator · pixel_7 · system-images;android-34;google_apis;x86_64 — booting"; !strings.Contains(stdout, want) {
		t.Errorf("stdout missing %q:\n%s", want, stdout)
	}
	for _, absent := range []string{"Device app:", "Device view:", "Device notes:"} {
		if strings.Contains(stdout, absent) {
			t.Errorf("stdout should not contain %q for a booting device without an app:\n%s", absent, stdout)
		}
	}
}

// TestViewCmd_DeviceNotRunning: an unspecified device state (VM not up yet)
// renders as "not running" rather than an empty word.
func TestViewCmd_DeviceNotRunning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{
			"id":"s-3","name":"ios-check","status":"SESSION_STATUS_PENDING",
			"device":{"spec":{"platform":"ios"},"state":"PREVIEW_DEVICE_STATE_UNSPECIFIED"}
		}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := "iOS simulator — not running"; !strings.Contains(stdout, want) {
		t.Errorf("stdout missing %q:\n%s", want, stdout)
	}
}

// TestViewCmd_JSONOutput_Device pins the additive `device` object in the
// stable JSON shape: snake_case keys, enums normalized to the same short
// words the human output prints, spec fields passed through.
func TestViewCmd_JSONOutput_Device(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{
			"id":"s-1","name":"ios-check","status":"SESSION_STATUS_RUNNING",
			"device":{
				"spec":{"platform":"ios","deviceModel":"iPhone 16","osVersion":"com.apple.CoreSimulator.SimRuntime.iOS-18-2"},
				"state":"PREVIEW_DEVICE_STATE_READY",
				"installStatus":"PREVIEW_INSTALL_STATUS_FAILED",
				"installReason":"bad artifact",
				"deviceNotes":"warm boot",
				"appName":"Demo","buildNumber":"42",
				"viewerUrl":"https://viewer.example.com/d/abc"
			}
		}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Device *struct {
			Spec struct {
				Platform    string `json:"platform"`
				DeviceModel string `json:"device_model"`
				OSVersion   string `json:"os_version"`
			} `json:"spec"`
			State         string `json:"state"`
			InstallStatus string `json:"install_status"`
			InstallReason string `json:"install_reason"`
			DeviceNotes   string `json:"device_notes"`
			AppName       string `json:"app_name"`
			BuildNumber   string `json:"build_number"`
			ViewerURL     string `json:"viewer_url"`
		} `json:"device"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if got.Device == nil {
		t.Fatalf("device object missing from JSON:\n%s", stdout)
	}
	d := got.Device
	if d.State != "ready" || d.InstallStatus != "failed" {
		t.Errorf("state/install_status = %q/%q, want ready/failed", d.State, d.InstallStatus)
	}
	if d.Spec.Platform != "ios" || d.Spec.DeviceModel != "iPhone 16" || d.Spec.OSVersion != "com.apple.CoreSimulator.SimRuntime.iOS-18-2" {
		t.Errorf("unexpected spec: %+v", d.Spec)
	}
	if d.InstallReason != "bad artifact" || d.DeviceNotes != "warm boot" || d.AppName != "Demo" || d.BuildNumber != "42" || d.ViewerURL != "https://viewer.example.com/d/abc" {
		t.Errorf("unexpected device fields: %+v", d)
	}
	if strings.Contains(stdout, "PREVIEW_") {
		t.Errorf("wire enum leaked into JSON output:\n%s", stdout)
	}
}

// TestViewCmd_JSONOutput_NoDevice: sessions without a device keep the
// pre-existing JSON shape — no `device` key at all.
func TestViewCmd_JSONOutput_NoDevice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_RUNNING"}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidSession}, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if _, has := got["device"]; has {
		t.Errorf("device key must be absent for a plain session: %v", got)
	}
}

func TestViewCmd_WatchRejectsJSON(t *testing.T) {
	// --watch + --output json must fail fast before any HTTP call (the JSON
	// contract is a single object, not a stream).
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("server should not be hit when --watch + json is rejected")
	}))
	defer srv.Close()

	_, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{"s-1", "--watch"}, output.JSON)
	if err == nil || !strings.Contains(err.Error(), "json") {
		t.Errorf("error = %v, want --watch/json incompatibility", err)
	}
}

func TestNotificationsCmd_HappyPath_MapsType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/ws-1/sessions/"+uuidSession+"/notifications" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"notifications":[
			{"id":"n-1","type":"SESSION_NOTIFICATION_TYPE_AGENT_STOPPED","title":"Agent stopped","createdAt":"2026-05-28T10:00:00Z"}
		]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newNotificationsCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"agent_stopped", "Agent stopped", "n-1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestNotificationsCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"notifications":[
			{"id":"n-1","type":"SESSION_NOTIFICATION_TYPE_AGENT_STOPPED","title":"Agent stopped"}
		]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newNotificationsCmd(), srv.URL, "ws-1", []string{uuidSession}, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got struct {
		Items []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if len(got.Items) != 1 || got.Items[0].Type != "agent_stopped" {
		t.Errorf("unexpected JSON items: %+v", got.Items)
	}
}

func TestNotificationsCmd_EmptyHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"notifications":[]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newNotificationsCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "No notifications.") {
		t.Errorf("expected empty-state message, got: %q", stdout)
	}
}

func TestNotificationsCmd_RejectsBadOrder(t *testing.T) {
	_, _, err := run(t, newNotificationsCmd(), "http://unused", "ws-1",
		[]string{"s-1", "--order", "sideways"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "asc") {
		t.Errorf("error = %v, want --order validation error", err)
	}
}

func TestNotificationsCmd_RejectsBadSince(t *testing.T) {
	_, _, err := run(t, newNotificationsCmd(), "http://unused", "ws-1",
		[]string{"s-1", "--since", "yesterday"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--since") {
		t.Errorf("error = %v, want --since parse error", err)
	}
}
