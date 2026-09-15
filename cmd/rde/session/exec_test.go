package session

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func TestBuildExecCommand(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		shell bool
		want  string
	}{
		{
			name: "default quotes each token so metacharacters stay literal",
			args: []string{"echo", "a; b"},
			want: `echo 'a; b'`,
		},
		{
			name: "default leaves simple argv untouched",
			args: []string{"npm", "test"},
			want: "npm test",
		},
		{
			name:  "shell mode joins a single quoted command verbatim",
			args:  []string{"cd repo && ls | head"},
			shell: true,
			want:  "cd repo && ls | head",
		},
		{
			name:  "shell mode joins multiple tokens with spaces, no quoting",
			args:  []string{"cd", "/x", "&&", "ls"},
			shell: true,
			want:  "cd /x && ls",
		},
		{
			// Shell mode does not escape single quotes — that is the remote
			// wrapper's job (see internalrde.buildLoginShellCmd). Here the
			// command is passed through verbatim, quotes and all.
			name:  "shell mode passes single quotes through unescaped",
			args:  []string{"grep -rn 'TODO' ."},
			shell: true,
			want:  "grep -rn 'TODO' .",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildExecCommand(tc.args, tc.shell); got != tc.want {
				t.Errorf("buildExecCommand(%q, shell=%v) = %q, want %q", tc.args, tc.shell, got, tc.want)
			}
		})
	}
}

// TestExecCmd_TimeoutFlagDefault pins that exec registers --timeout and that
// its default tracks the service-layer default (so a change to one without the
// other is caught).
func TestExecCmd_TimeoutFlagDefault(t *testing.T) {
	c := newExecCmd()
	if c.Flags().Lookup("timeout") == nil {
		t.Fatal("exec should register a --timeout flag")
	}
	got, err := c.Flags().GetDuration("timeout")
	if err != nil {
		t.Fatalf("GetDuration(timeout): %v", err)
	}
	if got != internalrde.DefaultExecuteTimeout {
		t.Errorf("--timeout default = %s, want %s", got, internalrde.DefaultExecuteTimeout)
	}
	if internalrde.DefaultExecuteTimeout != 10*time.Minute {
		t.Errorf("DefaultExecuteTimeout = %s, want 10m", internalrde.DefaultExecuteTimeout)
	}
}

func TestExecCmd_EnvFlagsRegistered(t *testing.T) {
	c := newExecCmd()
	envFlag := c.Flags().Lookup("env")
	if envFlag == nil {
		t.Fatal("exec should register an --env flag")
	}
	if envFlag.Value.Type() != "stringArray" {
		t.Errorf("--env type = %q, want stringArray (repeatable, no comma-splitting)", envFlag.Value.Type())
	}
	noFile, err := c.Flags().GetBool("no-env-file")
	if err != nil {
		t.Fatalf("GetBool(no-env-file): %v", err)
	}
	if noFile {
		t.Error("--no-env-file should default to false")
	}
}

// writeExecEnvDotfile drops a .bitrise/rde.yml into dir and chdirs there, so
// the test exercises the auto-read path without picking up any real dotfile
// in an ancestor of the package directory.
func writeExecEnvDotfile(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".bitrise"), 0o755); err != nil { //nolint:gosec // test-only tempdir, perms don't matter
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".bitrise", "rde.yml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
}

// terminatedSessionServer serves a GET for uuidSession whose status is
// terminated, so exec's Execute call fails cleanly after the env diagnostics
// have been printed (no SSH dial is attempted against a terminated session).
func terminatedSessionServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/sessions/"+uuidSession {
			_, _ = io.WriteString(w, `{"session":{"id":"s-1","name":"dev","status":"SESSION_STATUS_TERMINATED"}}`)
			return
		}
		t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestExecCmd_EnvFlagUnsetVarFailsFast pins that a --env NAME with no local
// value errors before any network call — the server fails the test on any
// request — and that the error names the variable.
func TestExecCmd_EnvFlagUnsetVarFailsFast(t *testing.T) {
	t.Chdir(t.TempDir()) // no dotfile in scope
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request before env validation: %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	_, _, err := run(t, newExecCmd(), srv.URL, "ws-1", []string{uuidSession, "--env", "BCLI_TEST_DEFINITELY_UNSET", "--", "echo", "hi"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "BCLI_TEST_DEFINITELY_UNSET") {
		t.Fatalf("err = %v, want error naming BCLI_TEST_DEFINITELY_UNSET", err)
	}
}

func TestExecCmd_DotfileWarningAndNotice(t *testing.T) {
	writeExecEnvDotfile(t, "exec:\n  env:\n    - BCLI_TEST_DEFINITELY_UNSET\n    - FOO=secret-value-xyz\n")
	srv := terminatedSessionServer(t)

	_, stderr, err := run(t, newExecCmd(), srv.URL, "ws-1", []string{uuidSession, "--", "echo", "hi"}, output.Human)
	if err == nil {
		t.Fatal("expected the exec against a terminated session to error")
	}
	if !strings.Contains(stderr, "BCLI_TEST_DEFINITELY_UNSET not set locally") {
		t.Errorf("stderr missing skip warning:\n%s", stderr)
	}
	if !strings.Contains(stderr, "Forwarding env: FOO") {
		t.Errorf("stderr missing forwarding notice:\n%s", stderr)
	}
	if strings.Contains(stderr, "secret-value-xyz") {
		t.Errorf("stderr leaks a forwarded value:\n%s", stderr)
	}
}

func TestExecCmd_NoEnvFileFlag(t *testing.T) {
	writeExecEnvDotfile(t, "exec:\n  env:\n    - BCLI_TEST_DEFINITELY_UNSET\n    - FOO=secret-value-xyz\n")
	srv := terminatedSessionServer(t)

	_, stderr, err := run(t, newExecCmd(), srv.URL, "ws-1", []string{uuidSession, "--no-env-file", "--", "echo", "hi"}, output.Human)
	if err == nil {
		t.Fatal("expected the exec against a terminated session to error")
	}
	for _, unwanted := range []string{"not set locally", "Forwarding env"} {
		if strings.Contains(stderr, unwanted) {
			t.Errorf("stderr should not mention %q with --no-env-file:\n%s", unwanted, stderr)
		}
	}
}

// TestExecCmd_QuietSuppressesNoticeNotWarning wraps exec in a parent command
// carrying the persistent --quiet flag (cmdutil.IsQuiet reads it from the
// root, which for a bare newExecCmd() is always false): with -q the
// forwarding notice disappears while the skip warning stays, per the output
// scheme's rule that warnings ignore --quiet.
func TestExecCmd_QuietSuppressesNoticeNotWarning(t *testing.T) {
	writeExecEnvDotfile(t, "exec:\n  env:\n    - BCLI_TEST_DEFINITELY_UNSET\n    - FOO=secret-value-xyz\n")
	srv := terminatedSessionServer(t)

	parent := &cobra.Command{Use: "root"}
	parent.PersistentFlags().BoolP("quiet", "q", false, "")
	parent.AddCommand(newExecCmd())

	_, stderr, err := run(t, parent, srv.URL, "ws-1", []string{"exec", uuidSession, "-q", "--", "echo", "hi"}, output.Human)
	if err == nil {
		t.Fatal("expected the exec against a terminated session to error")
	}
	if strings.Contains(stderr, "Forwarding env") {
		t.Errorf("-q should suppress the forwarding notice:\n%s", stderr)
	}
	if !strings.Contains(stderr, "BCLI_TEST_DEFINITELY_UNSET not set locally") {
		t.Errorf("-q must not suppress the skip warning:\n%s", stderr)
	}
}

// newRenderTestCmd returns a command with captured stdout/stderr so the
// render helpers can be exercised without a network.
func newRenderTestCmd() (*cobra.Command, *strings.Builder, *strings.Builder) {
	var out, errOut strings.Builder
	c := &cobra.Command{Use: "exec"}
	c.SetOut(&out)
	c.SetErr(&errOut)
	return c, &out, &errOut
}

// TestRenderExecResult_JSONPropagatesExitCode pins the bug where --output json
// returned nil for a failed remote command: the JSON envelope must still be the
// only thing on stdout, and the call must return an error (→ exit 1) with the
// explanation on stderr.
func TestRenderExecResult_JSONPropagatesExitCode(t *testing.T) {
	c, out, errOut := newRenderTestCmd()
	res := internalrde.ExecResult{ExitCode: 3, Stdout: "partial", Stderr: "boom"}

	err := renderExecResult(c, output.JSON, res, "")
	if err == nil {
		t.Fatal("expected an error for a non-zero remote exit in JSON mode")
	}
	if !strings.Contains(err.Error(), "exited with status 3") {
		t.Errorf("error = %q, want it to name status 3", err)
	}
	if !strings.Contains(out.String(), `"exit_code": 3`) || strings.Contains(out.String(), "remote command exited") {
		t.Errorf("stdout must carry only the JSON envelope, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "remote command exited with status 3") {
		t.Errorf("stderr = %q, want the exit explanation", errOut.String())
	}
	if !c.SilenceErrors {
		t.Error("cobra error echo must be silenced so the message is not printed twice")
	}
}

func TestRenderExecResult_JSONZeroExitIsNil(t *testing.T) {
	c, out, errOut := newRenderTestCmd()
	if err := renderExecResult(c, output.JSON, internalrde.ExecResult{Stdout: "ok"}, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"exit_code": 0`) {
		t.Errorf("stdout = %q, want JSON envelope", out.String())
	}
	if errOut.String() != "" {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}

func TestRenderExecResult_HumanAppendsHint(t *testing.T) {
	c, out, errOut := newRenderTestCmd()
	res := internalrde.ExecResult{ExitCode: 127, Stderr: "bash: cd x && ls: command not found\n"}
	err := renderExecResult(c, output.Human, res, shellHintText)
	if err == nil {
		t.Fatal("expected error")
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
	if !strings.Contains(errOut.String(), "command not found") || !strings.Contains(errOut.String(), "pass --shell before '--'") {
		t.Errorf("stderr = %q, want remote stderr followed by the --shell hint", errOut.String())
	}
}

func TestShellHint(t *testing.T) {
	cases := []struct {
		name string
		args []string
		sh   bool
		code int
		want bool
	}{
		{"single multi-word token, 127, no --shell → hint", []string{"cd x && ls"}, false, 127, true},
		{"single token with pipe, 127 → hint", []string{"ls|head"}, false, 127, true},
		{"single token with $(), 127 → hint", []string{"echo $(id)"}, false, 127, true},
		{"--shell set → no hint", []string{"cd x && ls"}, true, 127, false},
		{"exit 1 → no hint", []string{"cd x && ls"}, false, 1, false},
		{"exit 0 → no hint", []string{"cd x && ls"}, false, 0, false},
		{"several tokens → no hint", []string{"cd", "x", "&&", "ls"}, false, 127, false},
		{"plain missing binary → no hint", []string{"nosuchtool"}, false, 127, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shellHint(tc.args, tc.sh, tc.code) != ""
			if got != tc.want {
				t.Errorf("shellHint(%q, shell=%v, %d) hint=%v, want %v", tc.args, tc.sh, tc.code, got, tc.want)
			}
		})
	}
}
