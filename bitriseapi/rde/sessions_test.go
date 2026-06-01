package rde

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListSessions_PathAndParse(t *testing.T) {
	rs := newRecordingServer(t, `{"sessions":[
		{"id":"s1","name":"dev","status":"SESSION_STATUS_RUNNING","templateSnapshot":{"templateName":"tmpl","image":"osx"}},
		{"id":"s2","name":"old","status":"SESSION_STATUS_TERMINATED"}
	]}`)

	sessions, err := rs.client().ListSessions(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if rs.lastMethod != http.MethodGet {
		t.Errorf("method = %s, want GET", rs.lastMethod)
	}
	if want := "/v1/workspaces/ws-1/sessions"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}
	if sessions[0].TemplateSnapshot == nil || sessions[0].TemplateSnapshot.TemplateName != "tmpl" {
		t.Errorf("session[0] template snapshot = %+v", sessions[0].TemplateSnapshot)
	}
	// Wire format keeps the raw enum string — mapping to snake_case lives in internal/rde.
	if sessions[0].Status != "SESSION_STATUS_RUNNING" {
		t.Errorf("status = %q, want raw enum", sessions[0].Status)
	}
}

func TestGetSession_Path(t *testing.T) {
	rs := newRecordingServer(t, `{"session":{"id":"s1","name":"dev"}}`)

	sess, err := rs.client().GetSession(context.Background(), "ws-1", "s1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if want := "/v1/workspaces/ws-1/sessions/s1"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	if sess.ID != "s1" {
		t.Errorf("id = %q, want s1", sess.ID)
	}
}

func TestGetSession_EscapesSessionID(t *testing.T) {
	rs := newRecordingServer(t, `{"session":{"id":"a b"}}`)

	if _, err := rs.client().GetSession(context.Background(), "ws-1", "a b"); err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	// The raw request URI must carry the escaped form.
	if want := "/v1/workspaces/ws-1/sessions/a%20b"; rs.lastURI != want {
		t.Errorf("escaped URI = %s, want %s", rs.lastURI, want)
	}
}

func TestCreateSession_BodyAndResponse(t *testing.T) {
	rs := newRecordingServer(t, `{"session":{"id":"new","name":"dev","status":"SESSION_STATUS_PENDING"},
		"autoMappedInputs":[{"sessionInputKey":"gh","savedInputId":"sv-1"}]}`)

	mins := 30
	sess, mapped, err := rs.client().CreateSession(context.Background(), "ws-1", CreateSessionRequest{
		Name:                    "dev",
		TemplateID:              "tmpl-1",
		SessionInputs:           []SessionInputValue{{Key: "repo", Value: "app"}},
		AutoTerminateMinutes:    &mins,
		MapSavedToSessionInputs: true,
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if rs.lastMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", rs.lastMethod)
	}
	if want := "/v1/workspaces/ws-1/sessions"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}

	var sent CreateSessionRequest
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal sent body: %v", err)
	}
	if sent.TemplateID != "tmpl-1" {
		t.Errorf("sent templateId = %q, want tmpl-1", sent.TemplateID)
	}
	if sent.AutoTerminateMinutes == nil || *sent.AutoTerminateMinutes != 30 {
		t.Errorf("sent autoTerminateMinutes = %v, want pointer to 30", sent.AutoTerminateMinutes)
	}
	if !sent.MapSavedToSessionInputs {
		t.Error("sent mapSavedToSessionInputs = false, want true")
	}

	if sess.ID != "new" {
		t.Errorf("session id = %q, want new", sess.ID)
	}
	if len(mapped) != 1 || mapped[0].SessionInputKey != "gh" || mapped[0].SavedInputID != "sv-1" {
		t.Errorf("auto-mapped = %+v", mapped)
	}
}

func TestUpdateSession_OmitsUnsetPointerFields(t *testing.T) {
	rs := newRecordingServer(t, `{"session":{"id":"s1","name":"renamed"}}`)

	name := "renamed"
	if _, err := rs.client().UpdateSession(context.Background(), "ws-1", "s1", UpdateSessionRequest{Name: &name}); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}
	if rs.lastMethod != http.MethodPatch {
		t.Errorf("method = %s, want PATCH", rs.lastMethod)
	}

	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent["name"] != "renamed" {
		t.Errorf("name = %v, want renamed", sent["name"])
	}
	if _, ok := sent["description"]; ok {
		t.Errorf("description should be omitted, body = %s", rs.lastBody)
	}
	if _, ok := sent["autoTerminateMinutes"]; ok {
		t.Errorf("autoTerminateMinutes should be omitted, body = %s", rs.lastBody)
	}
}

func TestRestoreSession_Path(t *testing.T) {
	rs := newRecordingServer(t, `{"session":{"id":"s1","status":"SESSION_STATUS_STARTING"}}`)

	if _, err := rs.client().RestoreSession(context.Background(), "ws-1", "s1"); err != nil {
		t.Fatalf("RestoreSession: %v", err)
	}
	if rs.lastMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", rs.lastMethod)
	}
	if want := "/v1/workspaces/ws-1/sessions/s1/restore"; rs.lastPath != want {
		t.Errorf("path = %s, want %s (canonical /restore, not /start)", rs.lastPath, want)
	}
}

func TestTerminateSession_Path(t *testing.T) {
	rs := newRecordingServer(t, `{"session":{"id":"s1","status":"SESSION_STATUS_TERMINATED"}}`)

	if _, err := rs.client().TerminateSession(context.Background(), "ws-1", "s1"); err != nil {
		t.Fatalf("TerminateSession: %v", err)
	}
	if want := "/v1/workspaces/ws-1/sessions/s1/terminate"; rs.lastPath != want {
		t.Errorf("path = %s, want %s (canonical /terminate, not /stop)", rs.lastPath, want)
	}
}

func TestDeleteSession_Path(t *testing.T) {
	rs := newRecordingServer(t, ``)

	if err := rs.client().DeleteSession(context.Background(), "ws-1", "s1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if rs.lastMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", rs.lastMethod)
	}
	if want := "/v1/workspaces/ws-1/sessions/s1"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
}

func TestDeleteTerminatedSessions_PathAndCount(t *testing.T) {
	rs := newRecordingServer(t, `{"deletedCount":3}`)

	n, err := rs.client().DeleteTerminatedSessions(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("DeleteTerminatedSessions: %v", err)
	}
	if rs.lastMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", rs.lastMethod)
	}
	if want := "/v1/workspaces/ws-1/sessions:delete-terminated"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	if n != 3 {
		t.Errorf("deleted count = %d, want 3", n)
	}
}

func TestCompareSessionTemplate_PathAndParse(t *testing.T) {
	rs := newRecordingServer(t, `{
		"snapshot":{"templateName":"tmpl","image":"osx"},
		"current":{"templateName":"tmpl","image":"osx-edge"},
		"changedVariableKeys":["FOO"]
	}`)

	resp, err := rs.client().CompareSessionTemplate(context.Background(), "ws-1", "s1")
	if err != nil {
		t.Fatalf("CompareSessionTemplate: %v", err)
	}
	if want := "/v1/workspaces/ws-1/sessions/s1/template-diff"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	if resp.Snapshot == nil || resp.Current == nil {
		t.Fatalf("snapshot/current missing: %+v", resp)
	}
	if resp.Snapshot.Image != "osx" || resp.Current.Image != "osx-edge" {
		t.Errorf("images: snapshot=%q current=%q", resp.Snapshot.Image, resp.Current.Image)
	}
	if len(resp.ChangedVariableKeys) != 1 || resp.ChangedVariableKeys[0] != "FOO" {
		t.Errorf("changed keys = %+v", resp.ChangedVariableKeys)
	}
}

// TestSessions_ValidationGuards confirms every session method validates its
// required IDs before issuing an HTTP request.
func TestSessions_ValidationGuards(t *testing.T) {
	rs := newRecordingServer(t, `{}`)
	c := rs.client()
	ctx := context.Background()

	cases := map[string]func() error{
		"ListSessions/no-ws":           func() error { _, err := c.ListSessions(ctx, ""); return err },
		"GetSession/no-ws":             func() error { _, err := c.GetSession(ctx, "", "s1"); return err },
		"GetSession/no-session":        func() error { _, err := c.GetSession(ctx, "ws", ""); return err },
		"CreateSession/no-ws":          func() error { _, _, err := c.CreateSession(ctx, "", CreateSessionRequest{}); return err },
		"UpdateSession/no-ws":          func() error { _, err := c.UpdateSession(ctx, "", "s1", UpdateSessionRequest{}); return err },
		"UpdateSession/no-session":     func() error { _, err := c.UpdateSession(ctx, "ws", "", UpdateSessionRequest{}); return err },
		"RestoreSession/no-session":    func() error { _, err := c.RestoreSession(ctx, "ws", ""); return err },
		"TerminateSession/no-session":  func() error { _, err := c.TerminateSession(ctx, "ws", ""); return err },
		"DeleteSession/no-session":     func() error { return c.DeleteSession(ctx, "ws", "") },
		"DeleteTerminated/no-ws":       func() error { _, err := c.DeleteTerminatedSessions(ctx, ""); return err },
		"CompareTemplate/no-session":   func() error { _, err := c.CompareSessionTemplate(ctx, "ws", ""); return err },
		"CompareTemplate/no-workspace": func() error { _, err := c.CompareSessionTemplate(ctx, "", "s1"); return err },
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
	if rs.hits != 0 {
		t.Errorf("validation guards made %d HTTP call(s); should short-circuit", rs.hits)
	}
}
