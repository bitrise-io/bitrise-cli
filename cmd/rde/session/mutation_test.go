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

// uuidTemplate is a UUID-shaped template arg so ResolveTemplateID
// short-circuits without an extra ListTemplates call.
const uuidTemplate = "11111111-2222-3333-4444-555555555555"

// uuidSession is a UUID-shaped session arg. Real RDE session IDs are UUIDs,
// so passing one exercises the ResolveSessionID short-circuit (no extra
// ListSessions call) — the same path production hits when a user pastes an ID.
const uuidSession = "99999999-8888-7777-6666-555555555555"

func TestCreateCmd_HappyPath(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/workspaces/ws-1/sessions" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"session":{"id":"s-new","name":"dev","status":"SESSION_STATUS_PENDING"}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newCreateCmd(), srv.URL, "ws-1",
		[]string{"--template", uuidTemplate, "--name", "dev", "--input", "repo=my-app"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["templateId"] != uuidTemplate || gotBody["name"] != "dev" {
		t.Errorf("unexpected create body: %v", gotBody)
	}
	if !strings.Contains(stdout, "Session created") || !strings.Contains(stdout, "s-new") {
		t.Errorf("stdout missing create confirmation:\n%s", stdout)
	}
}

func TestCreateCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{"id":"s-new","name":"dev","status":"SESSION_STATUS_PENDING"},
			"autoMappedInputs":[{"sessionInputKey":"gh","savedInputId":"sv-1"}]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newCreateCmd(), srv.URL, "ws-1",
		[]string{"--template", uuidTemplate, "--name", "dev", "--map-saved-inputs"}, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got struct {
		Session struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"session"`
		AutoMapped []struct {
			SessionInputKey string `json:"session_input_key"`
		} `json:"auto_mapped_inputs"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if got.Session.ID != "s-new" || got.Session.Status != "pending" {
		t.Errorf("unexpected session JSON: %+v", got.Session)
	}
	if len(got.AutoMapped) != 1 || got.AutoMapped[0].SessionInputKey != "gh" {
		t.Errorf("unexpected auto-mapped JSON: %+v", got.AutoMapped)
	}
}

func TestCreateCmd_RequiresName(t *testing.T) {
	_, _, err := run(t, newCreateCmd(), "http://unused", "ws-1",
		[]string{"--template", uuidTemplate}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--name") {
		t.Errorf("error = %v, want --name required", err)
	}
}

func TestCreateCmd_AutoTerminateZeroIsSent(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"session":{"id":"s-new","name":"dev"}}`)
	}))
	defer srv.Close()

	// Explicitly setting --auto-terminate-minutes 0 must send 0 (disable),
	// distinct from omitting the flag (backend default).
	_, _, err := run(t, newCreateCmd(), srv.URL, "ws-1",
		[]string{"--template", uuidTemplate, "--name", "dev", "--auto-terminate-minutes", "0"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	v, ok := gotBody["autoTerminateMinutes"]
	if !ok {
		t.Fatalf("autoTerminateMinutes should be present when flag is set, body=%v", gotBody)
	}
	if v != float64(0) {
		t.Errorf("autoTerminateMinutes = %v, want 0", v)
	}
}

func TestCreateCmd_AutoTerminateOmittedWhenFlagUnset(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"session":{"id":"s-new","name":"dev"}}`)
	}))
	defer srv.Close()

	_, _, err := run(t, newCreateCmd(), srv.URL, "ws-1",
		[]string{"--template", uuidTemplate, "--name", "dev"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, ok := gotBody["autoTerminateMinutes"]; ok {
		t.Errorf("autoTerminateMinutes should be omitted when flag unset, body=%v", gotBody)
	}
}

func TestUpdateCmd_RequiresAField(t *testing.T) {
	_, _, err := run(t, newUpdateCmd(), "http://unused", "ws-1", []string{"s-1"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--name") {
		t.Errorf("error = %v, want at-least-one-field error", err)
	}
}

func TestUpdateCmd_OnlySetFieldsSent(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v1/workspaces/ws-1/sessions/"+uuidSession {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"renamed"}}`)
	}))
	defer srv.Close()

	_, _, err := run(t, newUpdateCmd(), srv.URL, "ws-1",
		[]string{uuidSession, "--name", "renamed"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["name"] != "renamed" {
		t.Errorf("name = %v, want renamed", gotBody["name"])
	}
	// --description / --auto-terminate-minutes weren't set, so they drop out.
	if _, ok := gotBody["description"]; ok {
		t.Errorf("description should be omitted, body=%v", gotBody)
	}
	if _, ok := gotBody["autoTerminateMinutes"]; ok {
		t.Errorf("autoTerminateMinutes should be omitted, body=%v", gotBody)
	}
}

func TestTerminateCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/workspaces/ws-1/sessions/"+uuidSession+"/terminate" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_TERMINATING"}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newTerminateCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "terminating") {
		t.Errorf("stdout missing status:\n%s", stdout)
	}
}

func TestRestoreCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/sessions/"+uuidSession:
			// Pre-flight disk-status check before the restore call.
			_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_TERMINATED","persistentDiskStatus":"PERSISTENT_DISK_STATUS_AVAILABLE"}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces/ws-1/sessions/"+uuidSession+"/restore":
			_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_STARTING"}}`)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	stdout, _, err := run(t, newRestoreCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "starting") {
		t.Errorf("stdout missing status:\n%s", stdout)
	}
}

func TestRestoreCmd_DiskUnavailableFailsFast(t *testing.T) {
	var restoreCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/sessions/"+uuidSession:
			_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_TERMINATED","persistentDiskStatus":"PERSISTENT_DISK_STATUS_UNAVAILABLE"}}`)
		case r.URL.Path == "/v1/workspaces/ws-1/sessions/"+uuidSession+"/restore":
			restoreCalled = true
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	_, _, err := run(t, newRestoreCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err == nil {
		t.Fatal("expected restore to fail when the persistent disk is unavailable")
	}
	if !strings.Contains(err.Error(), "no longer available") {
		t.Errorf("error missing reason: %v", err)
	}
	if restoreCalled {
		t.Error("restore endpoint should not be called when the disk is unavailable")
	}
}

func TestRestoreCmd_DiskExpiringSoonWarnsButProceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/sessions/"+uuidSession:
			_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_TERMINATED","persistentDiskStatus":"PERSISTENT_DISK_STATUS_UNAVAILABLE_SOON"}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces/ws-1/sessions/"+uuidSession+"/restore":
			_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_STARTING"}}`)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	stdout, stderr, err := run(t, newRestoreCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stderr, "unavailable soon") {
		t.Errorf("stderr missing expiry warning:\n%s", stderr)
	}
	if !strings.Contains(stdout, "starting") {
		t.Errorf("stdout missing status:\n%s", stdout)
	}
}

func TestDeleteCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/workspaces/ws-1/sessions/"+uuidSession {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	stdout, stderr, err := run(t, newDeleteCmd(), srv.URL, "ws-1", []string{uuidSession}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stderr, "Deleted session "+uuidSession) {
		t.Errorf("stderr missing confirmation: %q", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout should be empty for delete, got: %q", stdout)
	}
}

// TestDeleteCmd_ResolvesName: a non-UUID arg is treated as a session name,
// looked up via ListSessions, and the resolved ID is what actually gets
// deleted.
func TestDeleteCmd_ResolvesName(t *testing.T) {
	var deletedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/sessions":
			_, _ = io.WriteString(w, `{"sessions":[{"id":"s-9","name":"my-box"},{"id":"s-7","name":"other"}]}`)
		case r.Method == http.MethodDelete:
			deletedPath = r.URL.Path
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	_, stderr, err := run(t, newDeleteCmd(), srv.URL, "ws-1", []string{"my-box"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if deletedPath != "/v1/workspaces/ws-1/sessions/s-9" {
		t.Errorf("deleted path = %q, want the resolved id s-9", deletedPath)
	}
	if !strings.Contains(stderr, "Deleted session s-9") {
		t.Errorf("stderr should confirm deletion of the resolved id: %q", stderr)
	}
}

// TestDeleteCmd_AmbiguousNameError pins the non-uniqueness guard: when a name
// matches more than one session, delete refuses rather than guessing.
func TestDeleteCmd_AmbiguousNameError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/sessions" {
			_, _ = io.WriteString(w, `{"sessions":[{"id":"s-1","name":"dup"},{"id":"s-2","name":"dup"}]}`)
			return
		}
		t.Errorf("nothing should be deleted for an ambiguous name: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	_, _, err := run(t, newDeleteCmd(), srv.URL, "ws-1", []string{"dup"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error = %v, want ambiguous-name error", err)
	}
}

func TestDeleteTerminatedCmd_YesSkipsPrompt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/workspaces/ws-1/sessions:delete-terminated" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"deletedCount":3}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newDeleteTerminatedCmd(), srv.URL, "ws-1", []string{"--yes"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "Deleted 3 terminated session(s)") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestDeleteTerminatedCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"deletedCount":3}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newDeleteTerminatedCmd(), srv.URL, "ws-1", []string{"--yes"}, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got struct {
		DeletedCount int `json:"deleted_count"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if got.DeletedCount != 3 {
		t.Errorf("deleted_count = %d, want 3", got.DeletedCount)
	}
}

func TestDeleteTerminatedCmd_AbortsOnNo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("server should not be hit when the user declines confirmation")
	}))
	defer srv.Close()

	// No --yes; feed "n" to the confirmation prompt.
	c := newDeleteTerminatedCmd()
	c.SetIn(strings.NewReader("n\n"))
	_, _, err := run(t, c, srv.URL, "ws-1", nil, output.Human)
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Errorf("error = %v, want aborted", err)
	}
}

func TestDeleteTerminatedCmd_ProceedsOnYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"deletedCount":1}`)
	}))
	defer srv.Close()

	c := newDeleteTerminatedCmd()
	c.SetIn(strings.NewReader("y\n"))
	stdout, _, err := run(t, c, srv.URL, "ws-1", nil, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "Deleted 1 terminated session(s)") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}
