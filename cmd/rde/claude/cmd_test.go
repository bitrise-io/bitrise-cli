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

func TestBuildClaudeCommand(t *testing.T) {
	if got, want := buildClaudeCommand("repo"),
		"tmux new-session -A -s claude -c repo 'exec claude'"; got != want {
		t.Errorf("got  %q\n want %q", got, want)
	}
	if got, want := buildClaudeCommand("my repo"),
		"tmux new-session -A -s claude -c 'my repo' 'exec claude'"; got != want {
		t.Errorf("quoted dir:\n got  %q\n want %q", got, want)
	}
}

func TestParseClaudeAccessToken(t *testing.T) {
	if got, ok := parseClaudeAccessToken([]byte(`{"claudeAiOauth":{"accessToken":"tok-123","refreshToken":"r"}}`)); !ok || got != "tok-123" {
		t.Errorf("json blob: got %q ok=%v, want tok-123 true", got, ok)
	}
	if got, ok := parseClaudeAccessToken([]byte("  bare-token\n")); !ok || got != "bare-token" {
		t.Errorf("bare token: got %q ok=%v, want bare-token true", got, ok)
	}
	if _, ok := parseClaudeAccessToken([]byte(`{"claudeAiOauth":{"accessToken":""}}`)); ok {
		t.Error("empty accessToken JSON should not be ok")
	}
	if _, ok := parseClaudeAccessToken([]byte("")); ok {
		t.Error("empty input should not be ok")
	}
}

func TestExistingLocalCredentialEnv(t *testing.T) {
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-tok")
	t.Setenv("ANTHROPIC_API_KEY", "api-key")
	cred, ok := existingLocalCredential()
	if !ok || cred.EnvVar != "CLAUDE_CODE_OAUTH_TOKEN" || cred.Value != "oauth-tok" {
		t.Errorf("oauth env should win: %+v ok=%v", cred, ok)
	}

	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	cred, ok = existingLocalCredential()
	if !ok || cred.EnvVar != "ANTHROPIC_API_KEY" || cred.Value != "api-key" {
		t.Errorf("api key fallback: %+v ok=%v", cred, ok)
	}
}

func TestExtractToken(t *testing.T) {
	// Token surrounded by escape sequences, with terminal-restore codes as the
	// trailing output (the case that previously got saved as the "token").
	withEscapes := "\x1b[?2004hPaste here:\x1b[0m\nsk-ant-oat01-abc_DEF-123.xyz\n\x1b[>4m\x1b[<u\x1b[?1004l\x1b[?2031l\x1b[?2004l"
	if got := extractToken(withEscapes); got != "sk-ant-oat01-abc_DEF-123.xyz" {
		t.Errorf("with escapes: got %q", got)
	}
	// Pure escape-sequence garbage must not be mistaken for a token.
	if got := extractToken("\x1b[>4m\x1b[<u\x1b[?1004l\x1b[?2031l\x1b[?2004l"); got != "" {
		t.Errorf("escape-only: got %q, want empty", got)
	}
	// Fallback: a clean token-shaped final line with no sk-ant prefix.
	if got := extractToken("info\nABCDEF0123456789abcdefXYZ\n"); got != "ABCDEF0123456789abcdefXYZ" {
		t.Errorf("fallback: got %q", got)
	}
}

func TestBuildCloneCommand(t *testing.T) {
	const url = "git@github.com:org/repo.git"
	if got, want := buildCloneCommand(url, "repo", "main", false),
		"GIT_SSH_COMMAND='ssh -o StrictHostKeyChecking=accept-new' git clone --branch main git@github.com:org/repo.git repo"; got != want {
		t.Errorf("with branch:\n got  %q\n want %q", got, want)
	}
	if got, want := buildCloneCommand(url, "repo", "feature/x", true),
		"GIT_SSH_COMMAND='ssh -o StrictHostKeyChecking=accept-new' git clone git@github.com:org/repo.git repo"; got != want {
		t.Errorf("default branch:\n got  %q\n want %q", got, want)
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
