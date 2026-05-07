package yml

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func TestUpdateCmd_FromStdin(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/my-app/bitrise.yml" || r.Method != http.MethodPost {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer srv.Close()

	c := newUpdateCmd()
	stderr := &bytes.Buffer{}
	c.SetOut(io.Discard)
	c.SetErr(stderr)
	c.SetIn(strings.NewReader(sampleYML))
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stderr.String(), "updated successfully") {
		t.Errorf("stderr missing success message: %q", stderr.String())
	}

	var body map[string]any
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("invalid JSON body: %v\n%s", err, gotBody)
	}
	if _, ok := body["app_config_datastore_yaml"]; !ok {
		t.Errorf("request body missing app_config_datastore_yaml: %v", body)
	}
}

func TestUpdateCmd_EmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	c := newUpdateCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetIn(strings.NewReader(""))
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty-content error, got %v", err)
	}
}

func TestUpdateCmd_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bitrise.yml")
	if err := os.WriteFile(path, []byte(sampleYML), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer srv.Close()

	c := newUpdateCmd()
	stderr := &bytes.Buffer{}
	c.SetOut(io.Discard)
	c.SetErr(stderr)
	c.SetArgs([]string{"--file", path})
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
		AppSlug:    "my-app",
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stderr.String(), "updated successfully") {
		t.Errorf("stderr missing success message: %q", stderr.String())
	}
}
