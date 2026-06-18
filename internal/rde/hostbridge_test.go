package rde

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testCredential = "0123456789abcdef0123456789abcdef" // #nosec G101 -- test fixture, not a real secret

// newTestBridge returns a bridge with a fixed credential and a single open-vnc
// action backed by handle. It does not touch SSH — only handler() is exercised.
func newTestBridge(handle func(ctx context.Context, r *http.Request) (any, error)) *HostBridge {
	b := &HostBridge{
		Actions: map[string]HostAction{
			ActionOpenVNC: {Handle: handle},
		},
	}
	b.credential = testCredential
	return b
}

func okHandle(_ context.Context, _ *http.Request) (any, error) {
	return map[string]any{"opened": true}, nil
}

func authHeader() http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+testCredential)
	return h
}

func TestHostBridgeHandler(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		auth       string // raw Authorization header; "" means omit
		wantStatus int
	}{
		{"valid request", http.MethodPost, "/open-vnc", "Bearer " + testCredential, http.StatusOK},
		{"missing token", http.MethodPost, "/open-vnc", "", http.StatusUnauthorized},
		{"wrong token", http.MethodPost, "/open-vnc", "Bearer nope", http.StatusUnauthorized},
		{"non-bearer scheme", http.MethodPost, "/open-vnc", testCredential, http.StatusUnauthorized},
		{"unknown action", http.MethodPost, "/does-not-exist", "Bearer " + testCredential, http.StatusNotFound},
		{"wrong method", http.MethodGet, "/open-vnc", "Bearer " + testCredential, http.StatusMethodNotAllowed},
		// Auth is checked before routing, so an unknown path without a token is
		// still rejected as unauthorized, not 404.
		{"unknown action no token", http.MethodPost, "/does-not-exist", "", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newTestBridge(okHandle)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			rec := httptest.NewRecorder()
			b.handler().ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestHostBridgeHandlerRunsActionAndEncodesResult(t *testing.T) {
	called := false
	b := newTestBridge(func(_ context.Context, _ *http.Request) (any, error) {
		called = true
		return map[string]any{"opened": true, "address": "host:5900"}, nil
	})
	req := httptest.NewRequest(http.MethodPost, "/open-vnc", nil)
	req.Header = authHeader()
	rec := httptest.NewRecorder()
	b.handler().ServeHTTP(rec, req)

	if !called {
		t.Fatal("action handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["opened"] != true || got["address"] != "host:5900" {
		t.Errorf("response = %v, want opened=true address=host:5900", got)
	}
}

func TestHostBridgeHandlerActionErrorIs500(t *testing.T) {
	b := newTestBridge(func(_ context.Context, _ *http.Request) (any, error) {
		return nil, fmt.Errorf("viewer not installed")
	})
	req := httptest.NewRequest(http.MethodPost, "/open-vnc", nil)
	req.Header = authHeader()
	rec := httptest.NewRecorder()
	b.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(got["error"], "viewer not installed") {
		t.Errorf("error body = %q, want it to mention the action error", got["error"])
	}
}

func TestHostBridgeHandlerCapsRequestBody(t *testing.T) {
	// An action that reads the body fails on an oversized payload because the
	// middleware installs http.MaxBytesReader — the cap that protects future
	// body-taking actions.
	b := newTestBridge(func(_ context.Context, r *http.Request) (any, error) {
		if _, err := io.ReadAll(r.Body); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	})
	big := strings.NewReader(strings.Repeat("a", (64<<10)+1))
	req := httptest.NewRequest(http.MethodPost, "/open-vnc", big)
	req.Header = authHeader()
	rec := httptest.NewRecorder()
	b.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body read should fail past the cap)", rec.Code)
	}
}

func TestHostBridgeSkillHeaderHasFrontmatter(t *testing.T) {
	if !strings.Contains(hostBridgeSkillHeader, "description:") {
		t.Error("skill header is missing the description frontmatter that drives auto-invocation")
	}
}

func TestBuildSkillComposesOnlyRegisteredActions(t *testing.T) {
	// The skill must describe exactly the registered actions and nothing else, so
	// a session never advertises a capability it lacks.
	b := &HostBridge{Actions: map[string]HostAction{
		"alpha": {SkillSection: "## alpha\nsection-alpha"},
	}}
	skill := b.buildSkill()
	if !strings.Contains(skill, hostBridgeSkillHeader[:40]) {
		t.Error("composed skill does not start from the shared header")
	}
	if !strings.Contains(skill, "section-alpha") {
		t.Error("composed skill is missing the registered action's section")
	}
	if strings.Contains(skill, "section-beta") {
		t.Error("composed skill leaked an unregistered action's section")
	}
}

func TestBuildSkillIsOrderedAndSkipsEmptySections(t *testing.T) {
	b := &HostBridge{Actions: map[string]HostAction{
		"zebra":   {SkillSection: "## zebra"},
		"alpha":   {SkillSection: "## alpha"},
		"noskill": {SkillSection: ""},
	}}
	skill := b.buildSkill()
	ai, zi := strings.Index(skill, "## alpha"), strings.Index(skill, "## zebra")
	if ai < 0 || zi < 0 || ai > zi {
		t.Errorf("sections not present in name order (alpha=%d zebra=%d)", ai, zi)
	}
	if strings.Contains(skill, "noskill") {
		t.Error("an action with an empty section should contribute nothing")
	}
}

func TestShellQuoteRoundTrip(t *testing.T) {
	// remoteWriteFile relies on shellQuote to carry arbitrary content through a
	// remote shell intact. Run the actual command through bash and read it back.
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	contents := []string{
		`{"url":"http://127.0.0.1:5900","token":"deadbeef"}`,
		`{"x":"a'b'c"}`, // single quotes are the interesting case for POSIX quoting
		"line1\nline2",
	}
	for i, content := range contents {
		path := filepath.Join(t.TempDir(), fmt.Sprintf("out-%d", i))
		cmd := fmt.Sprintf("printf '%%s' %s > %s", shellQuote(content), shellQuote(path))
		// #nosec G204 -- running our shellQuote output through a real shell is the point of this test; cmd is built from in-test fixtures
		if out, err := exec.Command("bash", "-c", cmd).CombinedOutput(); err != nil {
			t.Fatalf("bash: %v: %s", err, out)
		}
		got, err := os.ReadFile(path) // #nosec G304 -- test temp path
		if err != nil {
			t.Fatalf("read back: %v", err)
		}
		if string(got) != content {
			t.Errorf("round-trip[%d] = %q, want %q", i, got, content)
		}
	}
}

func TestListenerPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close() //nolint:errcheck // test cleanup
	port, err := listenerPort(ln)
	if err != nil {
		t.Fatalf("listenerPort: %v", err)
	}
	if port <= 0 {
		t.Errorf("port = %d, want > 0", port)
	}
}
