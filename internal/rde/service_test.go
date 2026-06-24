package rde

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

// recordingServer captures the method, path, auth header, and body of the
// last request, and replies with a canned JSON body.
type recordingServer struct {
	srv        *httptest.Server
	lastMethod string
	lastPath   string
	lastAuth   string
	lastBody   []byte
}

func newRecordingServer(t *testing.T, response string) *recordingServer {
	t.Helper()
	rs := &recordingServer{}
	rs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.lastMethod = r.Method
		rs.lastPath = r.URL.Path
		rs.lastAuth = r.Header.Get("Authorization")
		rs.lastBody, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(rs.srv.Close)
	return rs
}

func (rs *recordingServer) service() *Service {
	return NewService(rdeapi.New(rs.srv.URL, "tok"))
}

func TestListSessions_PathAuthAndStatusMapping(t *testing.T) {
	rs := newRecordingServer(t, `{"sessions":[
		{"id":"s1","name":"dev","status":"SESSION_STATUS_RUNNING","templateSnapshot":{"templateName":"tmpl"}},
		{"id":"s2","name":"old","status":"SESSION_STATUS_TERMINATED"}
	]}`)

	sessions, err := rs.service().ListSessions(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if rs.lastMethod != http.MethodGet {
		t.Errorf("method = %s, want GET", rs.lastMethod)
	}
	if want := "/v1/workspaces/ws-1/sessions"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	if rs.lastAuth != "Bearer tok" {
		t.Errorf("auth = %q, want %q", rs.lastAuth, "Bearer tok")
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}
	if sessions[0].Status != "running" {
		t.Errorf("status[0] = %q, want running", sessions[0].Status)
	}
	if sessions[0].TemplateName != "tmpl" {
		t.Errorf("template_name[0] = %q, want tmpl", sessions[0].TemplateName)
	}
	if sessions[1].Status != "terminated" {
		t.Errorf("status[1] = %q, want terminated", sessions[1].Status)
	}
}

func TestCreateSession_BodyAndAutoMapped(t *testing.T) {
	rs := newRecordingServer(t, `{"session":{"id":"new","name":"dev","status":"SESSION_STATUS_PENDING"},
		"autoMappedInputs":[{"sessionInputKey":"gh","savedInputId":"sv-1"}]}`)

	mins := 0
	res, err := rs.service().CreateSession(context.Background(), "ws-1", CreateSessionRequest{
		Name:                 "dev",
		TemplateID:           "tmpl-1",
		SessionInputs:        []SessionInputValue{{Key: "repo", Value: "app"}},
		AutoTerminateMinutes: &mins,
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if want := "/v1/workspaces/ws-1/sessions"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	if rs.lastMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", rs.lastMethod)
	}

	var sent rdeapi.CreateSessionRequest
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal sent body: %v", err)
	}
	if sent.TemplateID != "tmpl-1" {
		t.Errorf("sent templateId = %q, want tmpl-1", sent.TemplateID)
	}
	if sent.AutoTerminateMinutes == nil || *sent.AutoTerminateMinutes != 0 {
		t.Errorf("sent autoTerminateMinutes = %v, want pointer to 0", sent.AutoTerminateMinutes)
	}
	if len(sent.SessionInputs) != 1 || sent.SessionInputs[0].Key != "repo" {
		t.Errorf("sent sessionInputs = %+v", sent.SessionInputs)
	}

	if res.Session.Status != "pending" {
		t.Errorf("status = %q, want pending", res.Session.Status)
	}
	if len(res.AutoMappedInputs) != 1 || res.AutoMappedInputs[0].SessionInputKey != "gh" {
		t.Errorf("auto-mapped = %+v", res.AutoMappedInputs)
	}
}

func TestCreateSession_TemplateLess(t *testing.T) {
	rs := newRecordingServer(t, `{"session":{"id":"new","name":"dev","status":"SESSION_STATUS_PENDING"}}`)

	if _, err := rs.service().CreateSession(context.Background(), "ws-1", CreateSessionRequest{
		Name:        "dev",
		Image:       "osx-sequoia-26",
		MachineType: "g2.mac.m2pro.6c-14g",
	}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	var sent rdeapi.CreateSessionRequest
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal sent body: %v", err)
	}
	if sent.TemplateID != "" {
		t.Errorf("sent templateId = %q, want empty for a template-less session", sent.TemplateID)
	}
	if sent.Image != "osx-sequoia-26" || sent.MachineType != "g2.mac.m2pro.6c-14g" {
		t.Errorf("sent image/machineType = %q/%q", sent.Image, sent.MachineType)
	}
}

func TestUpdateSession_OmitsUnsetFields(t *testing.T) {
	rs := newRecordingServer(t, `{"session":{"id":"s1","name":"renamed"}}`)

	name := "renamed"
	if _, err := rs.service().UpdateSession(context.Background(), "ws-1", "s1", UpdateSessionRequest{Name: &name}); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}
	// Only "name" should be present in the PATCH body — description and
	// autoTerminateMinutes are nil, so the omitempty pointer fields drop out.
	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := sent["description"]; ok {
		t.Errorf("description should be omitted, body = %s", rs.lastBody)
	}
	if _, ok := sent["autoTerminateMinutes"]; ok {
		t.Errorf("autoTerminateMinutes should be omitted, body = %s", rs.lastBody)
	}
	if sent["name"] != "renamed" {
		t.Errorf("name = %v, want renamed", sent["name"])
	}
}

func TestSavedInputs_AreUserScopedNotWorkspaceScoped(t *testing.T) {
	rs := newRecordingServer(t, `{"savedInputs":[{"id":"sv-1","key":"gh","isSecret":true,"value":"***"}]}`)

	inputs, err := rs.service().ListSavedInputs(context.Background())
	if err != nil {
		t.Fatalf("ListSavedInputs: %v", err)
	}
	if want := "/v1/saved-inputs"; rs.lastPath != want {
		t.Errorf("path = %s, want %s (saved inputs are user-scoped)", rs.lastPath, want)
	}
	if len(inputs) != 1 || !inputs[0].IsSecret {
		t.Fatalf("inputs = %+v", inputs)
	}
}

func TestAPIError_SurfacesMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"code":5,"message":"session not found"}`)
	}))
	t.Cleanup(srv.Close)
	svc := NewService(rdeapi.New(srv.URL, "tok"))

	_, err := svc.GetSession(context.Background(), "ws-1", "missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *rdeapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not *rdeapi.APIError: %T", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", apiErr.StatusCode)
	}
	if apiErr.Message != "session not found" {
		t.Errorf("message = %q, want %q", apiErr.Message, "session not found")
	}
}

func TestWaitForReady_PollsUntilNonProvisioningStatus(t *testing.T) {
	// Two polls: first returns PENDING, second returns RUNNING.
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = io.WriteString(w, `{"session":{"id":"s1","status":"SESSION_STATUS_PENDING"}}`)
		default:
			_, _ = io.WriteString(w, `{"session":{"id":"s1","status":"SESSION_STATUS_RUNNING"}}`)
		}
	}))
	t.Cleanup(srv.Close)

	svc := NewService(rdeapi.New(srv.URL, "tok"))
	// onPoll should see the live status on every poll, including the
	// intermediate provisioning state.
	var seen []string
	// 1ms interval keeps the test fast.
	sess, err := svc.WaitForReady(context.Background(), "ws-1", "s1", time.Millisecond, func(status string) {
		seen = append(seen, status)
	})
	if err != nil {
		t.Fatalf("WaitForReady: %v", err)
	}
	if sess.Status != "running" {
		t.Errorf("status = %q, want running", sess.Status)
	}
	if calls < 2 {
		t.Errorf("expected at least 2 polls, got %d", calls)
	}
	if len(seen) < 2 || seen[0] != "pending" || seen[len(seen)-1] != "running" {
		t.Errorf("onPoll statuses = %v, want pending…running", seen)
	}
}

func TestWaitForReady_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{"id":"s1","status":"SESSION_STATUS_PENDING"}}`)
	}))
	t.Cleanup(srv.Close)

	svc := NewService(rdeapi.New(srv.URL, "tok"))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := svc.WaitForReady(ctx, "ws-1", "s1", time.Millisecond, nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want a context error", err)
	}
}

func TestWaitForTerminated_PollsUntilSettledStatus(t *testing.T) {
	// Two polls: first returns TERMINATING, second returns TERMINATED.
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = io.WriteString(w, `{"session":{"id":"s1","status":"SESSION_STATUS_TERMINATING"}}`)
		default:
			_, _ = io.WriteString(w, `{"session":{"id":"s1","status":"SESSION_STATUS_TERMINATED"}}`)
		}
	}))
	t.Cleanup(srv.Close)

	svc := NewService(rdeapi.New(srv.URL, "tok"))
	// 1ms interval keeps the test fast.
	sess, err := svc.WaitForTerminated(context.Background(), "ws-1", "s1", time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForTerminated: %v", err)
	}
	if sess.Status != "terminated" {
		t.Errorf("status = %q, want terminated", sess.Status)
	}
	if calls < 2 {
		t.Errorf("expected at least 2 polls, got %d", calls)
	}
}

func TestWaitForTerminated_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{"id":"s1","status":"SESSION_STATUS_TERMINATING"}}`)
	}))
	t.Cleanup(srv.Close)

	svc := NewService(rdeapi.New(srv.URL, "tok"))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := svc.WaitForTerminated(ctx, "ws-1", "s1", time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want a context error", err)
	}
}

func TestResolveTemplateID_UUIDShortCircuits(t *testing.T) {
	// httptest server fails the test if anything reaches it — UUID input
	// should never trigger a list call.
	rs := newRecordingServer(t, "")
	id := "11111111-2222-3333-4444-555555555555"
	got, err := rs.service().ResolveTemplateID(context.Background(), "ws-1", id)
	if err != nil {
		t.Fatalf("ResolveTemplateID(uuid): %v", err)
	}
	if got != id {
		t.Errorf("got %q, want passthrough %q", got, id)
	}
	if rs.lastPath != "" {
		t.Errorf("UUID input made an HTTP call to %q (should short-circuit)", rs.lastPath)
	}
}

func TestResolveTemplateID_NameLookup(t *testing.T) {
	rs := newRecordingServer(t, `{"templates":[
		{"id":"t1","name":"Linux Dev","image":"ubuntu","machineType":"m1"},
		{"id":"t2","name":"macOS Dev","image":"osx","machineType":"m2"}
	]}`)
	got, err := rs.service().ResolveTemplateID(context.Background(), "ws-1", "macOS Dev")
	if err != nil {
		t.Fatalf("ResolveTemplateID(name): %v", err)
	}
	if got != "t2" {
		t.Errorf("got %q, want t2", got)
	}
}

func TestResolveTemplateID_AmbiguousNameError(t *testing.T) {
	rs := newRecordingServer(t, `{"templates":[
		{"id":"t1","name":"dev"},
		{"id":"t2","name":"DEV"}
	]}`)
	_, err := rs.service().ResolveTemplateID(context.Background(), "ws-1", "dev")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("err = %v, want ambiguous-name error", err)
	}
}

func TestResolveSessionID_UUIDShortCircuits(t *testing.T) {
	// httptest server fails the test if anything reaches it — UUID input
	// should never trigger a list call.
	rs := newRecordingServer(t, "")
	id := "11111111-2222-3333-4444-555555555555"
	got, err := rs.service().ResolveSessionID(context.Background(), "ws-1", id)
	if err != nil {
		t.Fatalf("ResolveSessionID(uuid): %v", err)
	}
	if got != id {
		t.Errorf("got %q, want passthrough %q", got, id)
	}
	if rs.lastPath != "" {
		t.Errorf("UUID input made an HTTP call to %q (should short-circuit)", rs.lastPath)
	}
}

func TestResolveSessionID_NameLookup(t *testing.T) {
	rs := newRecordingServer(t, `{"sessions":[
		{"id":"s1","name":"dev"},
		{"id":"s2","name":"empty-linux-box3"}
	]}`)
	got, err := rs.service().ResolveSessionID(context.Background(), "ws-1", "empty-linux-box3")
	if err != nil {
		t.Fatalf("ResolveSessionID(name): %v", err)
	}
	if got != "s2" {
		t.Errorf("got %q, want s2", got)
	}
}

func TestResolveSessionID_AmbiguousNameError(t *testing.T) {
	rs := newRecordingServer(t, `{"sessions":[
		{"id":"s1","name":"dev"},
		{"id":"s2","name":"DEV"}
	]}`)
	_, err := rs.service().ResolveSessionID(context.Background(), "ws-1", "dev")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("err = %v, want ambiguous-name error", err)
	}
}

func TestResolveSessionID_NoMatchError(t *testing.T) {
	rs := newRecordingServer(t, `{"sessions":[{"id":"s1","name":"dev"}]}`)
	_, err := rs.service().ResolveSessionID(context.Background(), "ws-1", "nope")
	if err == nil || !strings.Contains(err.Error(), "no session named") {
		t.Errorf("err = %v, want no-match error", err)
	}
}

func TestNilClientGuards(t *testing.T) {
	svc := NewService(nil)
	if _, err := svc.ListSessions(context.Background(), "ws"); err == nil {
		t.Error("ListSessions with nil client should error")
	}
	if _, err := svc.ListSavedInputs(context.Background()); err == nil {
		t.Error("ListSavedInputs with nil client should error")
	}
}

func TestStatusFromAPI(t *testing.T) {
	cases := map[string]string{
		"SESSION_STATUS_RUNNING":     "running",
		"SESSION_STATUS_TERMINATED":  "terminated",
		"SESSION_STATUS_UNSPECIFIED": "",
		"":                           "",
		"SESSION_STATUS_FUTURE_NEW":  "future_new", // forward-compat: unknown still readable
	}
	for in, want := range cases {
		if got := statusFromAPI(in); got != want {
			t.Errorf("statusFromAPI(%q) = %q, want %q", in, got, want)
		}
	}
}
