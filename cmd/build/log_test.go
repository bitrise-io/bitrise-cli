package build

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func TestLogCmd_StreamsLogChunks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestLogCmd_StreamsArchivedLog(t *testing.T) {
	rawSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ARCHIVED LOG CONTENT\n")
	}))
	defer rawSrv.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
