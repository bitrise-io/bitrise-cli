package claude

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
)

// ensureAgentHasKey is an rde-claude-only convenience. The in-session git
// clone authenticates via the forwarded local SSH agent (see exec_ssh.go), so
// an agent with no keys loaded means a private clone fails. If the agent is
// running but empty, add a key — chosen from the SSH config for the clone host
// (via `ssh -G`, which also yields the built-in defaults), falling back to the
// default key files. Entirely best-effort: any failure just means the clone
// may prompt or fail, which the user sees live.
func ensureAgentHasKey(ctx context.Context, progress func(string, ...any), cloneURL string) {
	// Agent forwarding only helps SSH remotes; HTTPS clones don't use it.
	if !isSSHCloneURL(cloneURL) {
		return
	}
	// ssh-add may prompt for a passphrase, so only attempt it with a real
	// terminal (the rest of rde claude needs one anyway).
	if !cmdutil.IsTerminal(os.Stdin) {
		return
	}

	switch agentState(ctx) {
	case agentHasKeys, agentUnknown:
		return
	case agentNoAgent:
		progress("No SSH agent running ($SSH_AUTH_SOCK unset); git clone of private repos in the session may fail.\n")
		return
	case agentEmpty:
		// fall through to add a key
	}

	key := pickIdentityFile(ctx, sshHostFromURL(cloneURL))
	if key == "" {
		progress("SSH agent has no keys and no key file was found to add; private clones may fail.\n")
		return
	}
	progress("Adding SSH key %s to the agent…\n", key)
	add := exec.CommandContext(ctx, "ssh-add", key) //nolint:gosec // G204: key is a local key-file path (from ssh -G or default files), passed as its own argv element — no shell, no injection
	add.Stdin = os.Stdin
	add.Stdout = os.Stderr // ssh-add writes prompts/results to stderr; keep stdout clean
	add.Stderr = os.Stderr
	_ = add.Run() // best-effort; the clone surfaces any remaining auth error
}

type agentStatus int

const (
	agentHasKeys agentStatus = iota
	agentEmpty
	agentNoAgent
	agentUnknown
)

// agentState classifies `ssh-add -l`: exit 0 = has identities, exit 1 = agent
// reachable but empty, exit 2 = no agent reachable.
func agentState(ctx context.Context) agentStatus {
	err := exec.CommandContext(ctx, "ssh-add", "-l").Run()
	if err == nil {
		return agentHasKeys
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		switch exitErr.ExitCode() {
		case 1:
			return agentEmpty
		case 2:
			return agentNoAgent
		}
	}
	return agentUnknown
}

// pickIdentityFile returns the first existing private key ssh would use for
// host (from `ssh -G`, which lists configured IdentityFile entries followed by
// the built-in defaults), falling back to the default key files directly when
// `ssh -G` is unavailable.
func pickIdentityFile(ctx context.Context, host string) string {
	var candidates []string
	if host != "" {
		candidates = append(candidates, sshConfigIdentityFiles(ctx, host)...)
	}
	candidates = append(candidates, defaultKeyFiles()...)
	for _, c := range candidates {
		if c != "" && fileExists(c) {
			return c
		}
	}
	return ""
}

// sshConfigIdentityFiles asks `ssh -G git@host` for the identity files ssh
// would try for that host, in order. Paths are returned with ~ expanded.
func sshConfigIdentityFiles(ctx context.Context, host string) []string {
	out, err := exec.CommandContext(ctx, "ssh", "-G", "git@"+host).Output() //nolint:gosec // G204: host comes from the repo's git remote, passed as its own argv element — no shell, no injection
	if err != nil {
		return nil
	}
	var files []string
	for line := range strings.SplitSeq(string(out), "\n") {
		if rest, ok := strings.CutPrefix(line, "identityfile "); ok {
			files = append(files, expandHome(strings.TrimSpace(rest)))
		}
	}
	return files
}

func defaultKeyFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	names := []string{"id_ed25519", "id_ecdsa", "id_rsa"}
	files := make([]string, 0, len(names))
	for _, n := range names {
		files = append(files, filepath.Join(home, ".ssh", n))
	}
	return files
}

// isSSHCloneURL reports whether u is an SSH-form git URL (git@host:… or
// ssh://…), the only form the forwarded agent can authenticate.
func isSSHCloneURL(u string) bool {
	return strings.HasPrefix(u, "git@") || strings.HasPrefix(u, "ssh://")
}

// sshHostFromURL extracts the host from a git clone URL (ssh or https form).
func sshHostFromURL(u string) string {
	s := u
	for _, p := range []string{"ssh://", "https://", "http://", "git://"} {
		s = strings.TrimPrefix(s, p)
	}
	if _, after, ok := strings.Cut(s, "@"); ok {
		s = after
	}
	if i := strings.IndexAny(s, ":/"); i >= 0 {
		s = s[:i]
	}
	return s
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
		}
	}
	return p
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
