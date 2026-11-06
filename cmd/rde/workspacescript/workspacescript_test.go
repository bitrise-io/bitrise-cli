package workspacescript

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

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdtest"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func TestMain(m *testing.M) { os.Exit(cmdtest.RunIsolated(m)) }

const scriptPath = "/v1/workspaces/ws-1/workspace-script"

// stored is the script as GET ?include_secrets=true returns it.
const stored = `{"workspaceScript":{"configured":true,"enabled":true,"script":"my-agent &\n",
	"variables":[
		{"key":"REGION","value":"eu"},
		{"key":"TOKEN","value":"s3cret","isSecret":true},
		{"key":"SIGNING_KEY","isSecret":true,"protected":true,"valueStored":true}
	],
	"updatedBy":"admin@example.com","updatedAt":"2026-11-01T10:00:00Z"}}`

// masked is the same script without include_secrets.
const masked = `{"workspaceScript":{"configured":true,"enabled":true,"script":"my-agent &\n",
	"variables":[
		{"key":"REGION","value":"eu"},
		{"key":"TOKEN","isSecret":true},
		{"key":"SIGNING_KEY","isSecret":true,"protected":true,"valueStored":true}
	],
	"updatedBy":"admin@example.com","updatedAt":"2026-11-01T10:00:00Z"}}`

// run drives c against a Resolved context pointing at srvURL.
func run(t *testing.T, c *cobra.Command, srvURL string, args []string, stdin string, format output.Format) (string, string, error) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(stderr)
	c.SetIn(strings.NewReader(stdin))
	c.SetArgs(args)
	c.SilenceUsage, c.SilenceErrors = true, true
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		RDEAPIBaseURL: srvURL,
		Token:         "tok",
		Output:        format,
		WorkspaceID:   "ws-1",
	}))
	err := c.Execute()
	return stdout.String(), stderr.String(), err
}

// fakeBackend answers GET with getBody (recording the query) and PUT by
// recording the raw body and answering putResp.
type fakeBackend struct {
	getBody  string
	putResp  string
	getQuery string
	putBody  []byte
	puts     int
}

func (f *fakeBackend) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != scriptPath {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			f.getQuery = r.URL.RawQuery
			_, _ = io.WriteString(w, f.getBody)
		case http.MethodPut:
			f.puts++
			f.putBody, _ = io.ReadAll(r.Body)
			_, _ = io.WriteString(w, f.putResp)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestViewCmd_MasksSecrets(t *testing.T) {
	f := &fakeBackend{getBody: masked}
	stdout, _, err := run(t, newViewCmd(), f.server(t).URL, nil, "", output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if f.getQuery != "" {
		t.Errorf("query = %q, want none without --show-secrets", f.getQuery)
	}
	for _, want := range []string{
		"enabled", "admin@example.com", "2026-11-01 10:00 UTC",
		"KEY", "VISIBILITY", "VALUE",
		"REGION", "plain", "eu",
		"TOKEN", "secret", "(hidden)",
		"SIGNING_KEY", "protected", "(set)",
		"my-agent &",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestViewCmd_ShowSecrets(t *testing.T) {
	f := &fakeBackend{getBody: stored}
	stdout, _, err := run(t, newViewCmd(), f.server(t).URL, []string{"--show-secrets"}, "", output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if f.getQuery != "include_secrets=true" {
		t.Errorf("query = %q, want include_secrets=true", f.getQuery)
	}
	if !strings.Contains(stdout, "s3cret") || strings.Contains(stdout, "(hidden)") {
		t.Errorf("secret not revealed:\n%s", stdout)
	}
	if !strings.Contains(stdout, "(set)") {
		t.Errorf("protected must stay write-only:\n%s", stdout)
	}
}

func TestViewCmd_DisabledAndUnconfigured(t *testing.T) {
	f := &fakeBackend{getBody: `{"workspaceScript":{"configured":true,"script":"x"}}`}
	stdout, _, err := run(t, newViewCmd(), f.server(t).URL, nil, "", output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "disabled") || !strings.Contains(stdout, "--workspace-script-test") {
		t.Errorf("disabled state not explained:\n%s", stdout)
	}

	f = &fakeBackend{getBody: `{"workspaceScript":{}}`}
	stdout, _, err = run(t, newViewCmd(), f.server(t).URL, nil, "", output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "No workspace script saved yet") {
		t.Errorf("unconfigured state not shown:\n%s", stdout)
	}
}

func TestViewCmd_JSONOutput(t *testing.T) {
	f := &fakeBackend{getBody: masked}
	stdout, _, err := run(t, newViewCmd(), f.server(t).URL, nil, "", output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if got["configured"] != true || got["enabled"] != true || got["script"] != "my-agent &\n" ||
		got["updated_by"] != "admin@example.com" || got["updated_at"] != "2026-11-01T10:00:00Z" {
		t.Errorf("json = %v", got)
	}
	vars, _ := got["variables"].([]any)
	if len(vars) != 3 {
		t.Fatalf("variables = %v", got["variables"])
	}
	want := []map[string]any{
		{"key": "REGION", "value": "eu", "visibility": "plain"},
		{"key": "TOKEN", "visibility": "secret"},
		{"key": "SIGNING_KEY", "visibility": "protected", "value_stored": true},
	}
	for i, w := range want {
		v, _ := vars[i].(map[string]any)
		if len(v) != len(w) {
			t.Errorf("variable %d = %v, want %v", i, v, w)
			continue
		}
		for k, wv := range w {
			if v[k] != wv {
				t.Errorf("variable %d %s = %v, want %v", i, k, v[k], wv)
			}
		}
	}
}

func TestViewCmd_ForbiddenIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"code":7,"message":"The workspace script is not available for this workspace."}`)
	}))
	defer srv.Close()
	_, _, err := run(t, newViewCmd(), srv.URL, nil, "", output.Human)
	if err == nil || !strings.Contains(err.Error(), "not available for this workspace") {
		t.Errorf("err = %v", err)
	}
}

// Changing only the script reads with secrets and sends everything else
// back as it was: enabled, the plain and secret values, and the protected
// variable with "" so its stored value is kept. The exact body is pinned.
func TestUpdateCmd_ScriptOnlyKeepsTheRest(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "ws.sh")
	if err := os.WriteFile(file, []byte("echo new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &fakeBackend{getBody: stored, putResp: masked}
	stdout, stderr, err := run(t, newUpdateCmd(), f.server(t).URL, []string{"--script-file", file}, "", output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if f.getQuery != "include_secrets=true" {
		t.Errorf("GET query = %q, want include_secrets=true", f.getQuery)
	}
	want := `{"enabled":true,"script":"echo new\n","variables":[` +
		`{"key":"REGION","value":"eu","isSecret":false,"protected":false},` +
		`{"key":"TOKEN","value":"s3cret","isSecret":true,"protected":false},` +
		`{"key":"SIGNING_KEY","value":"","isSecret":true,"protected":true}]}`
	if string(f.putBody) != want {
		t.Errorf("PUT body\n got %s\nwant %s", f.putBody, want)
	}
	// What was saved is printed — masked, as the PUT answers.
	if !strings.Contains(stdout, "REGION") || strings.Contains(stdout, "s3cret") {
		t.Errorf("stdout:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Saved workspace script") {
		t.Errorf("stderr missing the confirmation: %q", stderr)
	}
}

type putVar struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	IsSecret  bool   `json:"isSecret"`
	Protected bool   `json:"protected"`
}

type putBody struct {
	Enabled   bool     `json:"enabled"`
	Script    string   `json:"script"`
	Variables []putVar `json:"variables"`
}

func decodePut(t *testing.T, raw []byte) putBody {
	t.Helper()
	var b putBody
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("PUT body is not JSON: %v\n%s", err, raw)
	}
	return b
}

func TestUpdateCmd_SetAndRemoveVariables(t *testing.T) {
	f := &fakeBackend{getBody: stored, putResp: masked}
	_, _, err := run(t, newUpdateCmd(), f.server(t).URL, []string{
		"--set-var", "REGION=us",
		"--set-secret", "NEW_TOKEN=abc=def",
		"--set-protected", "TOKEN=rotated",
		"--remove-var", "SIGNING_KEY",
		"--disable",
	}, "", output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := decodePut(t, f.putBody)
	want := putBody{Enabled: false, Script: "my-agent &\n", Variables: []putVar{
		{Key: "REGION", Value: "us"},
		{Key: "TOKEN", Value: "rotated", IsSecret: true, Protected: true},
		{Key: "NEW_TOKEN", Value: "abc=def", IsSecret: true},
	}}
	if got.Enabled != want.Enabled || got.Script != want.Script || len(got.Variables) != len(want.Variables) {
		t.Fatalf("PUT = %+v, want %+v", got, want)
	}
	for i := range want.Variables {
		if got.Variables[i] != want.Variables[i] {
			t.Errorf("variable %d = %+v, want %+v", i, got.Variables[i], want.Variables[i])
		}
	}
}

// A protected variable turned plain needs its new value, which the flag
// carries; the rest of the set is kept.
func TestUpdateCmd_VisibilityChange(t *testing.T) {
	f := &fakeBackend{getBody: stored, putResp: masked}
	if _, _, err := run(t, newUpdateCmd(), f.server(t).URL, []string{"--set-var", "SIGNING_KEY=public", "--set-secret", "REGION=eu"}, "", output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := decodePut(t, f.putBody)
	want := []putVar{
		{Key: "REGION", Value: "eu", IsSecret: true},
		{Key: "TOKEN", Value: "s3cret", IsSecret: true},
		{Key: "SIGNING_KEY", Value: "public"},
	}
	if len(got.Variables) != len(want) {
		t.Fatalf("variables = %+v", got.Variables)
	}
	for i := range want {
		if got.Variables[i] != want[i] {
			t.Errorf("variable %d = %+v, want %+v", i, got.Variables[i], want[i])
		}
	}
	if !got.Enabled {
		t.Error("enabled must be kept")
	}
}

func TestUpdateCmd_ScriptFromStdin(t *testing.T) {
	f := &fakeBackend{getBody: `{"workspaceScript":{}}`, putResp: `{"workspaceScript":{"configured":true,"script":"from stdin\n"}}`}
	if _, _, err := run(t, newUpdateCmd(), f.server(t).URL, []string{"--script-file", "-"}, "from stdin\n", output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := `{"enabled":false,"script":"from stdin\n","variables":[]}`; string(f.putBody) != want {
		t.Errorf("PUT body = %s, want %s", f.putBody, want)
	}
}

func TestUpdateCmd_JSONOutput(t *testing.T) {
	f := &fakeBackend{getBody: stored, putResp: masked}
	stdout, _, err := run(t, newUpdateCmd(), f.server(t).URL, []string{"--enable"}, "", output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if got["enabled"] != true {
		t.Errorf("json = %v", got)
	}
}

// Flag errors fail before any request: nothing to do, both --enable and
// --disable, malformed KEY=VALUE.
func TestUpdateCmd_RejectsBadFlags(t *testing.T) {
	cases := map[string]struct {
		args []string
		want string
	}{
		"no flags":           {nil, "nothing to update"},
		"enable and disable": {[]string{"--enable", "--disable"}, "none of the others can be"},
		"malformed var":      {[]string{"--set-var", "NOVALUE"}, "expected KEY=VALUE"},
		"empty key":          {[]string{"--set-secret", "=v"}, "expected KEY=VALUE"},
		"missing file":       {[]string{"--script-file", filepath.Join(t.TempDir(), "nope.sh")}, "read script"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("backend must not be called")
			}))
			defer srv.Close()
			_, _, err := run(t, newUpdateCmd(), srv.URL, tc.args, "", output.Human)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// Changes that only make sense against the stored script are checked after
// the read, and nothing is saved.
func TestUpdateCmd_RemoveUnknownVariableSavesNothing(t *testing.T) {
	f := &fakeBackend{getBody: stored, putResp: masked}
	_, _, err := run(t, newUpdateCmd(), f.server(t).URL, []string{"--remove-var", "NOPE"}, "", output.Human)
	if err == nil || !strings.Contains(err.Error(), "no variable NOPE") {
		t.Errorf("err = %v", err)
	}
	if f.puts != 0 {
		t.Errorf("PUT sent %d time(s), want none", f.puts)
	}
}
