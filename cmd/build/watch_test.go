package build

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

// watchStubServer serves a finished+archived build so Service.Watch takes the
// archived branch (stream log, then return the final View) without polling.
func watchStubServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ARCHIVED LOG LINE\n")
	}))
	t.Cleanup(raw.Close)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apps/my-app/builds/b-1/log":
			_, _ = io.WriteString(w, `{"is_archived":true,"expiring_raw_log_url":"`+raw.URL+`"}`)
		case "/apps/my-app/builds/b-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":{"slug":"b-1","build_number":7,"status":`+strconv.Itoa(status)+`,"triggered_at":"2026-05-06T10:00:00Z"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestWatchCmd_JSONWritesRecordToStdoutLogsToStderr(t *testing.T) {
	srv := watchStubServer(t, 1) // success

	c := newWatchCmd()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(stderr)
	c.SetArgs([]string{"b-1"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.JSON,
		AppSlug:    "my-app",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var rec map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rec); err != nil {
		t.Fatalf("stdout not valid JSON: %v\n%s", err, stdout.String())
	}
	if rec["status"] != "success" {
		t.Errorf("expected status success in stdout JSON, got %v", rec["status"])
	}
	if !bytes.Contains(stderr.Bytes(), []byte("ARCHIVED LOG LINE")) {
		t.Errorf("expected log lines on stderr, got %q", stderr.String())
	}
	if bytes.Contains(stdout.Bytes(), []byte("ARCHIVED LOG LINE")) {
		t.Errorf("log lines leaked into stdout JSON: %q", stdout.String())
	}
}

func TestWatchCmd_JSONFailedBuildExitsNonZero(t *testing.T) {
	srv := watchStubServer(t, 2) // failed

	c := newWatchCmd()
	c.SilenceUsage = true // production root sets this; detached test cmd must too
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"b-1"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.JSON,
		AppSlug:    "my-app",
	}))

	err := c.Execute()
	if err == nil {
		t.Fatal("expected non-zero exit (error) for failed build")
	}
	var rec map[string]any
	if jerr := json.Unmarshal(stdout.Bytes(), &rec); jerr != nil {
		t.Fatalf("stdout not valid JSON: %v\n%s", jerr, stdout.String())
	}
	if rec["status"] != "failed" {
		t.Errorf("expected status failed in stdout JSON, got %v", rec["status"])
	}
}
