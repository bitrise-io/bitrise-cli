package build

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdtest"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func TestLogCmd_StreamsLogChunks(t *testing.T) {
	srv := httptest.NewServer(cmdtest.AppPassthrough(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/my-app/builds/b-1/log" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"is_archived":false,"log_chunks":[{"chunk":"line one\n","position":0},{"chunk":"line two\n","position":1}]}`)
	}))
	defer srv.Close()

	c := newLogCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"b-1"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "line one") || !strings.Contains(out, "line two") {
		t.Errorf("expected log chunks in stdout, got: %q", out)
	}
}

func TestLogCmd_OrdersOutOfPositionChunks(t *testing.T) {
	// The API may return in-progress log chunks out of position order; the
	// CLI must sort them so the log reads top-to-bottom.
	srv := httptest.NewServer(cmdtest.AppPassthrough(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"is_archived":false,"log_chunks":[{"chunk":"two\n","position":1},{"chunk":"zero\n","position":0},{"chunk":"three\n","position":3},{"chunk":"one\n","position":2}]}`)
	}))
	defer srv.Close()

	c := newLogCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"b-1"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got, want := stdout.String(), "zero\ntwo\none\nthree\n"; got != want {
		t.Errorf("chunks not ordered by position:\n got %q\nwant %q", got, want)
	}
}

func TestLogCmd_WaitPolls_ThenPrintsLog(t *testing.T) {
	// First View call returns in-progress; second returns success.
	var viewCalls atomic.Int32
	srv := httptest.NewServer(cmdtest.AppPassthrough(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apps/my-app/builds/b-1":
			n := int(viewCalls.Add(1))
			if n == 1 {
				_, _ = io.WriteString(w, `{"data":{"slug":"b-1","build_number":5,"status":0,"triggered_at":"2026-05-06T10:00:00Z"}}`)
			} else {
				_, _ = io.WriteString(w, `{"data":{"slug":"b-1","build_number":5,"status":1,"triggered_at":"2026-05-06T10:00:00Z"}}`)
			}
		case "/apps/my-app/builds/b-1/log":
			_, _ = io.WriteString(w, `{"is_archived":false,"log_chunks":[{"chunk":"done\n","position":0}]}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := newLogCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(stderr)
	c.SetArgs([]string{"b-1", "--wait", "--interval", "1ms"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// Header written to stderr while waiting.
	if !strings.Contains(stderr.String(), "Waiting for build") {
		t.Errorf("stderr missing wait header: %q", stderr.String())
	}
	// Log printed to stdout after build finished.
	if !strings.Contains(stdout.String(), "done") {
		t.Errorf("stdout missing log content: %q", stdout.String())
	}
}

func TestLogCmd_WaitSkipsPolling_WhenAlreadyFinished(t *testing.T) {
	var viewCalls atomic.Int32
	srv := httptest.NewServer(cmdtest.AppPassthrough(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apps/my-app/builds/b-1":
			viewCalls.Add(1)
			_, _ = io.WriteString(w, `{"data":{"slug":"b-1","build_number":5,"status":1,"triggered_at":"2026-05-06T10:00:00Z"}}`)
		case "/apps/my-app/builds/b-1/log":
			_, _ = io.WriteString(w, `{"is_archived":false,"log_chunks":[{"chunk":"all good\n","position":0}]}`)
		}
	}))
	defer srv.Close()

	c := newLogCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(stderr)
	c.SetArgs([]string{"b-1", "--wait", "--interval", "1ms"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// Build was already finished — no wait header, no polling loop.
	if strings.Contains(stderr.String(), "Waiting for build") {
		t.Errorf("should not print wait header for finished build: %q", stderr.String())
	}
	if n := int(viewCalls.Load()); n != 1 {
		t.Errorf("expected exactly 1 View call, got %d", n)
	}
	if !strings.Contains(stdout.String(), "all good") {
		t.Errorf("stdout missing log: %q", stdout.String())
	}
}

func TestLogCmd_StreamsArchivedLog(t *testing.T) {
	rawSrv := httptest.NewServer(cmdtest.AppPassthrough(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ARCHIVED LOG CONTENT\n")
	}))
	defer rawSrv.Close()

	srv := httptest.NewServer(cmdtest.AppPassthrough(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"is_archived":true,"expiring_raw_log_url":"`+rawSrv.URL+`"}`)
	}))
	defer srv.Close()

	c := newLogCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"b-1"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "ARCHIVED LOG CONTENT") {
		t.Errorf("expected archived log in stdout, got: %q", stdout.String())
	}
}
