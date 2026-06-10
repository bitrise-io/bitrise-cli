package build

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func TestTriggerCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/my-app/builds" || r.Method != http.MethodPost {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"build_slug":"new-1","build_number":100,"build_url":"https://app.bitrise.io/build/new-1","triggered_workflow":"primary"}`)
	}))
	defer srv.Close()

	c := newTriggerCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))
	c.SetArgs([]string{"--workflow", "primary"})

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Build triggered") {
		t.Errorf("stdout missing 'Build triggered': %q", out)
	}
}

func TestTriggerCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"build_slug":"new-1","build_number":100,"build_url":"https://app.bitrise.io/build/new-1","triggered_workflow":"primary"}`)
	}))
	defer srv.Close()

	c := newTriggerCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.JSON,
		AppSlug:    "my-app",
	}))
	c.SetArgs([]string{"--workflow", "primary"})

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if got["id"] != "new-1" || got["build_number"] != float64(100) {
		t.Errorf("unexpected JSON: %v", got)
	}
}

func TestTriggerCmd_InvalidEnvJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	c := newTriggerCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))
	c.SetArgs([]string{"--env", "not-json"})

	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "--env") {
		t.Errorf("expected --env JSON parse error, got %v", err)
	}
}

func TestTriggerCmd_WaitAndWatchMutuallyExclusive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	c := newTriggerCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))
	c.SetArgs([]string{"--wait", "--watch"})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when --wait and --watch are both set")
	}
}

func TestTriggerCmd_Wait_BlocksAndExits(t *testing.T) {
	// Trigger returns a build; two View calls: first in-progress, then success.
	var viewCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/apps/my-app/builds" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"build_slug":"b-1","build_number":5,"build_url":"https://app.bitrise.io/build/b-1","triggered_workflow":"primary"}`)
		case r.URL.Path == "/apps/my-app/builds/b-1":
			n := int(viewCalls.Add(1))
			if n == 1 {
				_, _ = io.WriteString(w, `{"data":{"slug":"b-1","build_number":5,"status":0,"triggered_at":"2026-05-06T10:00:00Z"}}`)
			} else {
				_, _ = io.WriteString(w, `{"data":{"slug":"b-1","build_number":5,"status":1,"triggered_at":"2026-05-06T10:00:00Z"}}`)
			}
		}
	}))
	defer srv.Close()

	c := newTriggerCmd()
	stderr := &bytes.Buffer{}
	c.SetOut(io.Discard)
	c.SetErr(stderr)
	c.SetArgs([]string{"--workflow", "primary", "--wait", "--interval", "1ms"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stderr.String(), "Waiting for build") {
		t.Errorf("stderr missing wait header: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "finished") {
		t.Errorf("stderr missing finish line: %q", stderr.String())
	}
}

func TestTriggerCmd_Wait_FailedBuildReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/apps/my-app/builds" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"build_slug":"b-1","build_number":5}`)
		case r.URL.Path == "/apps/my-app/builds/b-1":
			_, _ = io.WriteString(w, `{"data":{"slug":"b-1","build_number":5,"status":2,"triggered_at":"2026-05-06T10:00:00Z"}}`)
		}
	}))
	defer srv.Close()

	c := newTriggerCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"--workflow", "primary", "--wait", "--interval", "1ms"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "failed") {
		t.Errorf("expected 'failed' error, got %v", err)
	}
}

func TestTriggerCmd_Wait_FailedBuildJSONWritesRecordAndErrors(t *testing.T) {
	// Regression: with --output json a failed build must still write the build
	// record to stdout AND return a non-zero error so CI scripts can gate on it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/apps/my-app/builds" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"build_slug":"b-1","build_number":5}`)
		case r.URL.Path == "/apps/my-app/builds/b-1":
			_, _ = io.WriteString(w, `{"data":{"slug":"b-1","build_number":5,"status":2,"triggered_at":"2026-05-06T10:00:00Z"}}`)
		}
	}))
	defer srv.Close()

	c := newTriggerCmd()
	c.SilenceUsage = true // production root sets this; detached test cmd must too
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"--workflow", "primary", "--wait", "--interval", "1ms"})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.JSON,
		AppSlug:    "my-app",
	}))

	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "failed") {
		t.Errorf("expected 'failed' error in JSON mode, got %v", err)
	}
	var rec map[string]any
	if jerr := json.Unmarshal(stdout.Bytes(), &rec); jerr != nil {
		t.Fatalf("stdout not valid JSON: %v\n%s", jerr, stdout.String())
	}
	if rec["id"] != "b-1" {
		t.Errorf("expected build record on stdout, got %v", rec)
	}
}

func TestTriggerCmd_DefaultsBranchToMain(t *testing.T) {
	var gotBranch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if bp, ok := body["build_params"].(map[string]any); ok {
			gotBranch, _ = bp["branch"].(string)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"build_slug":"x","build_number":1}`)
	}))
	defer srv.Close()

	c := newTriggerCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBranch != "main" {
		t.Errorf("expected default branch 'main', got %q", gotBranch)
	}
}
