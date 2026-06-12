package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-cli/internal/config"
)

// run executes a fresh api command against srvURL with the given args,
// returning stdout, stderr, and the execution error.
func run(t *testing.T, srvURL string, args []string, stdin string) (string, string, error) {
	t.Helper()
	c := NewCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(stderr)
	c.SetIn(strings.NewReader(stdin))
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		Output:     "human",
		APIBaseURL: srvURL,
		Token:      "tok",
	}))
	c.SetArgs(args)
	err := c.Execute()
	return stdout.String(), stderr.String(), err
}

func TestAPICmd_GETWritesRawBody(t *testing.T) {
	const body = `{"data":{"username":"alice"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/me" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)

	stdout, _, err := run(t, srv.URL, []string{"/me"}, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// Non-TTY stdout (a buffer) → raw passthrough, byte-for-byte.
	if stdout != body {
		t.Errorf("stdout = %q, want %q", stdout, body)
	}
}

func TestAPICmd_Include(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Trace", "abc")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(srv.Close)

	stdout, _, err := run(t, srv.URL, []string{"/me", "-i"}, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.HasPrefix(stdout, "HTTP 200 OK\n") {
		t.Errorf("stdout missing status line:\n%s", stdout)
	}
	if !strings.Contains(stdout, "X-Trace: abc") {
		t.Errorf("stdout missing header:\n%s", stdout)
	}
	if !strings.Contains(stdout, `{"ok":true}`) {
		t.Errorf("stdout missing body:\n%s", stdout)
	}
}

func TestAPICmd_Non2xxPrintsBodyAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"not found"}`)
	}))
	t.Cleanup(srv.Close)

	stdout, _, err := run(t, srv.URL, []string{"/nope"}, "")
	if err == nil {
		t.Fatal("expected a non-zero (error) result for HTTP 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %v, want it to mention 404", err)
	}
	if !strings.Contains(stdout, "not found") {
		t.Errorf("stdout should still carry the error body, got %q", stdout)
	}
}

func TestAPICmd_PostFieldsDefaultsMethodAndSendsJSON(t *testing.T) {
	var gotMethod string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)

	// No -X: presence of -f must default the method to POST.
	_, _, err := run(t, srv.URL, []string{"/apps", "-f", "title=Widget"}, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST (defaulted from -f)", gotMethod)
	}
	var obj map[string]string
	_ = json.Unmarshal(gotBody, &obj)
	if obj["title"] != "Widget" {
		t.Errorf("body = %s", gotBody)
	}
}

func TestAPICmd_InputFromStdin(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)

	_, _, err := run(t, srv.URL, []string{"/apps/x/builds", "-X", "POST", "--input", "-"}, `{"build":1}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody != `{"build":1}` {
		t.Errorf("body = %q, want stdin payload", gotBody)
	}
}

func TestAPICmd_AllMergesPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("next") {
		case "":
			_, _ = io.WriteString(w, `{"data":[{"id":"a"}],"paging":{"next":"p2"}}`)
		case "p2":
			_, _ = io.WriteString(w, `{"data":[{"id":"b"}],"paging":{"next":""}}`)
		default:
			t.Errorf("unexpected cursor")
		}
	}))
	t.Cleanup(srv.Close)

	stdout, _, err := run(t, srv.URL, []string{"/apps", "--all"}, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var env struct {
		Data []map[string]string `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout)
	}
	if len(env.Data) != 2 {
		t.Errorf("merged %d items, want 2:\n%s", len(env.Data), stdout)
	}
}

func TestAPICmd_FieldAndInputMutuallyExclusive(t *testing.T) {
	_, _, err := run(t, "https://example.invalid", []string{"/apps", "-f", "a=b", "--input", "-"}, "")
	if err == nil {
		t.Fatal("expected mutually-exclusive flag error")
	}
}

func TestAPICmd_RequiresPath(t *testing.T) {
	_, _, err := run(t, "https://example.invalid", []string{}, "")
	if err == nil || !strings.Contains(err.Error(), "PATH") {
		t.Errorf("expected missing PATH error, got %v", err)
	}
}
