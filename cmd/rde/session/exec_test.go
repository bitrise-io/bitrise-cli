package session

import "testing"

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
