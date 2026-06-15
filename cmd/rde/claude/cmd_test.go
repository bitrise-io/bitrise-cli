package claude

import "testing"

func TestGitSSHURL(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"ssh passthrough", "git@github.com:org/repo.git", "git@github.com:org/repo.git"},
		{"ssh scheme passthrough", "ssh://git@github.com/org/repo.git", "ssh://git@github.com/org/repo.git"},
		{"github https rewrite", "https://github.com/org/repo.git", "git@github.com:org/repo.git"},
		{"github https no suffix", "https://github.com/org/repo", "git@github.com:org/repo.git"},
		{"gitlab https rewrite", "https://gitlab.com/group/sub/repo.git", "git@gitlab.com:group/sub/repo.git"},
		{"bitbucket https rewrite", "https://bitbucket.org/team/repo", "git@bitbucket.org:team/repo.git"},
		{"https with user prefix", "https://user@github.com/org/repo.git", "git@github.com:org/repo.git"},
		{"unknown host left as-is", "https://git.example.com/org/repo.git", "https://git.example.com/org/repo.git"},
		{"trailing slash", "https://github.com/org/repo/", "git@github.com:org/repo.git"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := gitSSHURL(tc.in); got != tc.want {
				t.Errorf("gitSSHURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSSHHostFromURL(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"ssh scp form", "git@github.com:org/repo.git", "github.com"},
		{"ssh scheme", "ssh://git@github.com/org/repo.git", "github.com"},
		{"https form", "https://gitlab.com/group/repo.git", "gitlab.com"},
		{"https with user", "https://user@github.com/org/repo", "github.com"},
		{"git scheme", "git://example.com/repo.git", "example.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sshHostFromURL(tc.in); got != tc.want {
				t.Errorf("sshHostFromURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsSSHCloneURL(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"git@github.com:org/repo.git", true},
		{"ssh://git@github.com/org/repo.git", true},
		{"https://github.com/org/repo.git", false},
		{"git://example.com/repo.git", false},
	} {
		if got := isSSHCloneURL(tc.in); got != tc.want {
			t.Errorf("isSSHCloneURL(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseAgentVar(t *testing.T) {
	out := "SSH_AUTH_SOCK=/tmp/ssh-abc/agent.123; export SSH_AUTH_SOCK;\n" +
		"SSH_AGENT_PID=456; export SSH_AGENT_PID;\n" +
		"echo Agent pid 456;\n"
	if got := parseAgentVar(out, "SSH_AUTH_SOCK"); got != "/tmp/ssh-abc/agent.123" {
		t.Errorf("SSH_AUTH_SOCK = %q, want /tmp/ssh-abc/agent.123", got)
	}
	if got := parseAgentVar(out, "SSH_AGENT_PID"); got != "456" {
		t.Errorf("SSH_AGENT_PID = %q, want 456", got)
	}
	if got := parseAgentVar(out, "MISSING"); got != "" {
		t.Errorf("MISSING = %q, want empty", got)
	}
}

func TestRepoDirFromURL(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"ssh form", "git@github.com:org/repo.git", "repo"},
		{"https form", "https://github.com/org/repo.git", "repo"},
		{"no git suffix", "https://github.com/org/repo", "repo"},
		{"nested path", "https://gitlab.com/group/sub/repo.git", "repo"},
		{"trailing slash", "https://github.com/org/repo/", "repo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := repoDirFromURL(tc.in); got != tc.want {
				t.Errorf("repoDirFromURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
