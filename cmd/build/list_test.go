package build

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdtest"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func runListCmd(t *testing.T, srvURL string, args []string, format output.Format) (stdout, stderr string, err error) {
	t.Helper()
	c := newListCmd()
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	c.SetOut(stdoutBuf)
	c.SetErr(stderrBuf)
	c.SetArgs(args)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srvURL,
		Token:      "tok",
		Output:     format,
		AppSlug:    "my-app",
	}))
	err = c.Execute()
	return stdoutBuf.String(), stderrBuf.String(), err
}

func TestListCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(cmdtest.AppPassthrough(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/my-app/builds" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"data":[
				{"slug":"b-1","build_number":42,"status":1,"branch":"main","triggered_workflow":"primary","triggered_at":"2026-05-06T10:00:00Z"}
			],"paging":{}}`)
	}))
	defer srv.Close()

	stdout, _, err := runListCmd(t, srv.URL, nil, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"42", "success", "main", "primary", "b-1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestListCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(cmdtest.AppPassthrough(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"slug":"b-1","build_number":42,"status":1,"branch":"main","triggered_workflow":"primary","triggered_at":"2026-05-06T10:00:00Z"}],"paging":{}}`)
	}))
	defer srv.Close()

	stdout, _, err := runListCmd(t, srv.URL, nil, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	items, _ := got["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item, _ := items[0].(map[string]any)
	if item["id"] != "b-1" || item["status"] != "success" {
		t.Errorf("unexpected item: %v", item)
	}
}

func TestListCmd_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(cmdtest.AppPassthrough(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[],"paging":{}}`)
	}))
	defer srv.Close()

	stdout, _, err := runListCmd(t, srv.URL, nil, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "No builds found") {
		t.Errorf("expected empty-state message, got: %q", stdout)
	}
}

func TestListCmd_AllAndCursorMutuallyExclusive(t *testing.T) {
	srv := httptest.NewServer(cmdtest.AppPassthrough(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	_, _, err := runListCmd(t, srv.URL, []string{"--all", "--cursor", "tok"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--all") {
		t.Errorf("expected --all/--cursor conflict error, got %v", err)
	}
}

func TestListCmd_InvalidAfterFlag(t *testing.T) {
	srv := httptest.NewServer(cmdtest.AppPassthrough(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	_, _, err := runListCmd(t, srv.URL, []string{"--after", "not-a-time"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--after") {
		t.Errorf("expected --after parse error, got %v", err)
	}
}
