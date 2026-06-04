package session

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/internal/output"
)

// testSessionID is UUID-shaped so ResolveSessionID short-circuits without a
// ListSessions round-trip (which would otherwise hit our log-streaming server).
const testSessionID = "11111111-2222-3333-4444-555555555555"

// logsCmd returns a logs command with SilenceUsage set, mirroring what the
// production root does — so a runtime error doesn't dump usage into the test's
// captured stdout.
func logsCmd() *cobra.Command {
	c := newLogsCmd()
	c.SilenceUsage = true
	return c
}

// logFrame is the success wire frame for one chunk, newline-terminated.
func logFrame(content string) string {
	b, _ := json.Marshal(content)
	return `{"result":{"logContent":` + string(b) + `}}` + "\n"
}

func writeFrames(w http.ResponseWriter, frames ...string) {
	for _, f := range frames {
		_, _ = w.Write([]byte(f))
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
	}
}

func TestLogsCmd_SnapshotStreamsToStdout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if want := "/v1/workspaces/ws-1/sessions/11111111-2222-3333-4444-555555555555/logs/2"; r.URL.Path != want { // startup -> 2
			t.Errorf("path = %s, want %s", r.URL.Path, want)
		}
		writeFrames(w, logFrame("hello\n"), logFrame("world\n"))
	}))
	defer srv.Close()

	stdout, _, err := run(t, logsCmd(), srv.URL, "ws-1", []string{testSessionID, "--stage", "startup"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if stdout != "hello\nworld\n" {
		t.Errorf("stdout = %q, want raw concatenated log", stdout)
	}
}

func TestLogsCmd_SnapshotNotReady404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":5,"message":"logs not available yet for this stage"}`))
	}))
	defer srv.Close()

	stdout, stderr, err := run(t, logsCmd(), srv.URL, "ws-1", []string{testSessionID, "--stage", "warmup"}, output.Human)
	if err == nil {
		t.Fatal("expected non-zero exit (error) on 404")
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "available yet") || !strings.Contains(stderr, "--follow") {
		t.Errorf("stderr = %q, want a friendly 'not available yet … --follow' message", stderr)
	}
}

func TestLogsCmd_FollowRetriesPast404ThenStreams(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			// First connect: stage hasn't produced output yet.
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":5,"message":"logs not available yet for this stage"}`))
			return
		}
		writeFrames(w, logFrame("now streaming\n"))
	}))
	defer srv.Close()

	stdout, stderr, err := run(t, logsCmd(), srv.URL, "ws-1",
		[]string{testSessionID, "--stage", "startup", "--follow", "--retry-interval", "1ms"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if stdout != "now streaming\n" {
		t.Errorf("stdout = %q, want streamed content after retry", stdout)
	}
	if !strings.Contains(stderr, "Streaming") {
		t.Errorf("stderr = %q, want the follow header", stderr)
	}
	if got := atomic.LoadInt32(&hits); got < 2 {
		t.Errorf("server hits = %d, want >= 2 (404 then stream)", got)
	}
}

func TestLogsCmd_RejectsJSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("should not hit the network when --output json is rejected")
	}))
	defer srv.Close()

	_, _, err := run(t, logsCmd(), srv.URL, "ws-1", []string{testSessionID, "--stage", "startup"}, output.JSON)
	if err == nil {
		t.Fatal("expected error rejecting --output json")
	}
	if !strings.Contains(err.Error(), "json") {
		t.Errorf("error = %q, want it to mention json", err.Error())
	}
}

func TestLogsCmd_RequiresStage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("should not hit the network when --stage is missing")
	}))
	defer srv.Close()

	_, _, err := run(t, logsCmd(), srv.URL, "ws-1", []string{testSessionID}, output.Human)
	if err == nil {
		t.Fatal("expected error when --stage is not set")
	}
	if !strings.Contains(err.Error(), "stage") {
		t.Errorf("error = %q, want it to mention the required stage flag", err.Error())
	}
}

func TestLogsCmd_BadStageErrorsBeforeNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("should not hit the network for an invalid --stage")
	}))
	defer srv.Close()

	_, stderr, err := run(t, logsCmd(), srv.URL, "ws-1", []string{testSessionID, "--stage", "bogus"}, output.Human)
	if err == nil {
		t.Fatal("expected error for invalid --stage")
	}
	if strings.Contains(stderr, "Streaming") {
		t.Errorf("stderr = %q, header should not print before validation fails", stderr)
	}
}
