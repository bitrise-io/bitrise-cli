package session

import (
	"testing"
	"time"

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
