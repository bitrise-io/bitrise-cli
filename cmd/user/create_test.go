package user

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

// stubServer mounts a /users/sign_up + /users handler that always returns
// the canned account JSON. Returns the test server.
func stubServer(t *testing.T, statusOnCreate int, createBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/sign_up":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<meta name="csrf-token" content="t" />`)
		case "/users":
			w.WriteHeader(statusOnCreate)
			_, _ = io.WriteString(w, createBody)
		default:
			http.NotFound(w, r)
		}
	}))
}

// runCreate is a thin harness that drives newCreateCmd with the given args
// and stdin, returning stdout, stderr, and the RunE error.
func runCreate(t *testing.T, args []string, stdin string, baseURL string) (stdout, stderr string, err error) {
	t.Helper()
	c := newCreateCmd()
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	c.SetOut(stdoutBuf)
	c.SetErr(stderrBuf)
	c.SetIn(strings.NewReader(stdin))
	c.SetArgs(args)
	ctx := config.WithResolved(context.Background(), config.Resolved{
		WebBaseURL: baseURL,
		Output:     "human",
	})
	c.SetContext(ctx)
	err = c.Execute()
	return stdoutBuf.String(), stderrBuf.String(), err
}

func TestCreate_HappyPathHumanOutput(t *testing.T) {
	srv := stubServer(t, http.StatusCreated, `{"slug":"u-1","email":"a@b.io","username":"alice","first_name":"A","last_name":"L","confirmed_at":null}`)
	defer srv.Close()

	stdout, _, err := runCreate(t,
		[]string{"--email", "a@b.io", "--username", "alice", "--first-name", "A", "--last-name", "L", "--password-stdin"},
		"supersecret\n",
		srv.URL,
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "Account created") {
		t.Fatalf("stdout missing success line: %q", stdout)
	}
	if !strings.Contains(stdout, "a@b.io") || !strings.Contains(stdout, "alice") {
		t.Fatalf("stdout missing account fields: %q", stdout)
	}
	if !strings.Contains(stdout, "auth login --email a@b.io") {
		t.Fatalf("stdout missing next-step hint: %q", stdout)
	}
}

func TestCreate_JSONOutputProducesParseableUserRecord(t *testing.T) {
	srv := stubServer(t, http.StatusCreated, `{"slug":"u-1","email":"a@b.io","username":"alice","first_name":"A","last_name":"L","confirmed_at":null}`)
	defer srv.Close()

	c := newCreateCmd()
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	c.SetOut(stdoutBuf)
	c.SetErr(stderrBuf)
	c.SetIn(strings.NewReader("supersecret\n"))
	c.SetArgs([]string{"--email", "a@b.io", "--username", "alice", "--password-stdin"})
	ctx := config.WithResolved(context.Background(), config.Resolved{
		WebBaseURL: srv.URL,
		Output:     "json",
	})
	c.SetContext(ctx)
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(stdoutBuf.Bytes(), &got); err != nil {
		t.Fatalf("stdout not valid JSON: %v\n%s", err, stdoutBuf.String())
	}
	if got["email"] != "a@b.io" || got["username"] != "alice" {
		t.Fatalf("unexpected JSON: %v", got)
	}
}

func TestCreate_ServerErrorSurfacesFieldDetails(t *testing.T) {
	srv := stubServer(t, http.StatusUnprocessableEntity, `{"errors":{"email":[{"error":"taken"}]}}`)
	defer srv.Close()

	_, _, err := runCreate(t,
		[]string{"--email", "a@b.io", "--username", "alice", "--password-stdin"},
		"supersecret\n",
		srv.URL,
	)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "email: taken") {
		t.Fatalf("error %q missing field detail", err.Error())
	}
}

func TestCreate_RequiresPassword(t *testing.T) {
	_, _, err := runCreate(t,
		[]string{"--email", "a@b.io", "--username", "alice", "--password-stdin"},
		"\n",
		"http://unused.invalid",
	)
	if err == nil || !strings.Contains(err.Error(), "password is empty") {
		t.Fatalf("expected password-empty error, got %v", err)
	}
}
