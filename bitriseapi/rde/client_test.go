package rde

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// parseQuery decodes a recorded raw query string, failing the test on a
// malformed value.
func parseQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	v, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("parse query %q: %v", raw, err)
	}
	return v
}

// recordingServer spins up an httptest server, captures the last request
// (method, path, query, auth, headers, body), and replies with a canned
// status + body. It mirrors the fakeServer helper in bitriseapi/apps_test.go,
// adapted for the RDE client's Bearer-auth + camelCase wire format.
type recordingServer struct {
	t          *testing.T
	srv        *httptest.Server
	status     int
	response   string
	lastMethod string
	lastPath   string
	lastURI    string // raw, still-escaped path + query
	lastQuery  string
	lastAuth   string
	lastBody   []byte
	lastHeader http.Header
	hits       int
}

func newRecordingServer(t *testing.T, response string) *recordingServer {
	t.Helper()
	rs := &recordingServer{t: t, status: http.StatusOK, response: response}
	rs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.hits++
		rs.lastMethod = r.Method
		rs.lastPath = r.URL.Path
		rs.lastURI = r.RequestURI
		rs.lastQuery = r.URL.RawQuery
		rs.lastAuth = r.Header.Get("Authorization")
		rs.lastHeader = r.Header.Clone()
		rs.lastBody, _ = io.ReadAll(r.Body)
		if rs.status != http.StatusOK {
			w.WriteHeader(rs.status)
		}
		_, _ = io.WriteString(w, rs.response)
	}))
	t.Cleanup(rs.srv.Close)
	return rs
}

func (rs *recordingServer) client() *Client {
	return New(rs.srv.URL, "tok")
}

func TestNew_TrimsTrailingSlashFromBaseURL(t *testing.T) {
	c := New("https://api.bitrise.io/rde/", "tok")
	if c.baseURL != "https://api.bitrise.io/rde" {
		t.Errorf("baseURL = %q, want trailing slash trimmed", c.baseURL)
	}
}

func TestDo_SetsRequiredHeaders(t *testing.T) {
	rs := newRecordingServer(t, `{"savedInputs":[]}`)

	if _, err := rs.client().ListSavedInputs(context.Background()); err != nil {
		t.Fatalf("ListSavedInputs: %v", err)
	}
	checks := map[string]string{
		"Authorization":    "Bearer tok",
		"Accept":           "application/json",
		"User-Agent":       UserAgent,
		"X-Request-Source": RequestSource,
	}
	for h, want := range checks {
		if got := rs.lastHeader.Get(h); got != want {
			t.Errorf("header %s = %q, want %q", h, got, want)
		}
	}
}

func TestGetRequest_HasNoContentType(t *testing.T) {
	rs := newRecordingServer(t, `{"savedInputs":[]}`)

	if _, err := rs.client().ListSavedInputs(context.Background()); err != nil {
		t.Fatalf("ListSavedInputs: %v", err)
	}
	// getJSON sends no body, so Content-Type must be absent.
	if ct := rs.lastHeader.Get("Content-Type"); ct != "" {
		t.Errorf("GET Content-Type = %q, want empty", ct)
	}
}

func TestPostRequest_SetsJSONContentType(t *testing.T) {
	rs := newRecordingServer(t, `{"savedInput":{"id":"x","key":"k"}}`)

	if _, err := rs.client().CreateSavedInput(context.Background(), CreateSavedInputRequest{Key: "k", Value: "v"}); err != nil {
		t.Fatalf("CreateSavedInput: %v", err)
	}
	if ct := rs.lastHeader.Get("Content-Type"); ct != "application/json" {
		t.Errorf("POST Content-Type = %q, want application/json", ct)
	}
}

func TestAPIError_ExtractsMessageFromEnvelope(t *testing.T) {
	rs := newRecordingServer(t, `{"code":5,"message":"session not found"}`)
	rs.status = http.StatusNotFound

	_, err := rs.client().ListSavedInputs(context.Background())
	if err == nil {
		t.Fatal("expected error on 404")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not *APIError: %T", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
	if apiErr.Message != "session not found" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "session not found")
	}
	if apiErr.Body != "" {
		t.Errorf("Body = %q, want empty when message was parsed", apiErr.Body)
	}
	if want := "RDE API 404: session not found"; apiErr.Error() != want {
		t.Errorf("Error() = %q, want %q", apiErr.Error(), want)
	}
}

func TestAPIError_IncludesFieldViolations(t *testing.T) {
	rs := newRecordingServer(t, `{"code":3,"message":"Bad request.","details":[{"@type":"type.googleapis.com/google.rpc.BadRequest","fieldViolations":[{"field":"session_inputs","description":"missing required input: BUILD_TOKEN"}]}]}`)
	rs.status = http.StatusBadRequest

	_, err := rs.client().ListSavedInputs(context.Background())
	if err == nil {
		t.Fatal("expected error on 400")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not *APIError: %T", err)
	}
	if got, want := len(apiErr.Violations), 1; got != want {
		t.Fatalf("len(Violations) = %d, want %d", got, want)
	}
	if want := "missing required input: BUILD_TOKEN"; apiErr.Violations[0] != want {
		t.Errorf("Violations[0] = %q, want %q", apiErr.Violations[0], want)
	}
	if apiErr.Body != "" {
		t.Errorf("Body = %q, want empty when violations were parsed", apiErr.Body)
	}
	if want := "RDE API 400: Bad request.: missing required input: BUILD_TOKEN"; apiErr.Error() != want {
		t.Errorf("Error() = %q, want %q", apiErr.Error(), want)
	}
}

func TestAPIError_FieldViolationFallsBackToFieldName(t *testing.T) {
	rs := newRecordingServer(t, `{"details":[{"@type":"type.googleapis.com/google.rpc.BadRequest","fieldViolations":[{"field":"name"}]}]}`)
	rs.status = http.StatusBadRequest

	_, err := rs.client().ListSavedInputs(context.Background())
	if err == nil {
		t.Fatal("expected error on 400")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not *APIError: %T", err)
	}
	// No message, no description — the field name carries the only signal,
	// so the raw body must not be used as a fallback.
	if want := "RDE API 400: name"; apiErr.Error() != want {
		t.Errorf("Error() = %q, want %q", apiErr.Error(), want)
	}
}

func TestAPIError_FallsBackToRawBody(t *testing.T) {
	rs := newRecordingServer(t, "upstream exploded")
	rs.status = http.StatusInternalServerError

	_, err := rs.client().ListSavedInputs(context.Background())
	if err == nil {
		t.Fatal("expected error on 500")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not *APIError: %T", err)
	}
	// No JSON message field, so the raw body is preserved.
	if apiErr.Message != "" {
		t.Errorf("Message = %q, want empty", apiErr.Message)
	}
	if apiErr.Body != "upstream exploded" {
		t.Errorf("Body = %q, want %q", apiErr.Body, "upstream exploded")
	}
	if want := "RDE API 500: upstream exploded"; apiErr.Error() != want {
		t.Errorf("Error() = %q, want %q", apiErr.Error(), want)
	}
}

func TestAPIError_BareStatusWhenNoMessageOrBody(t *testing.T) {
	e := &APIError{StatusCode: http.StatusBadGateway}
	if want := "RDE API 502"; e.Error() != want {
		t.Errorf("Error() = %q, want %q", e.Error(), want)
	}
}

func TestContextCancellation(t *testing.T) {
	rs := newRecordingServer(t, `{"savedInputs":[]}`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := rs.client().ListSavedInputs(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 3, "hel…"},
		{"", 3, ""},
	}
	for _, c := range cases {
		if got := truncate(c.in, c.max); got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
	}
}

func TestWsPath(t *testing.T) {
	if got := wsPath("ws-1", "/sessions"); got != "/v1/workspaces/ws-1/sessions" {
		t.Errorf("wsPath = %q", got)
	}
	// Missing leading slash is tolerated.
	if got := wsPath("ws-1", "sessions"); got != "/v1/workspaces/ws-1/sessions" {
		t.Errorf("wsPath (no leading slash) = %q", got)
	}
	// Workspace IDs are path-escaped.
	if got := wsPath("a b", "/x"); got != "/v1/workspaces/a%20b/x" {
		t.Errorf("wsPath (escaping) = %q", got)
	}
}

func TestUserPath(t *testing.T) {
	if got := userPath("/saved-inputs"); got != "/v1/saved-inputs" {
		t.Errorf("userPath = %q", got)
	}
	if got := userPath("saved-inputs"); got != "/v1/saved-inputs" {
		t.Errorf("userPath (no leading slash) = %q", got)
	}
}
