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
// an agent with no keys loaded means a private clone fails. It:
//   - starts a temporary agent if none is running (and returns a cleanup that
//     kills it on exit);
//   - adds a key when the agent is empty — chosen from the SSH config for the
//     clone host (via `ssh -G`, which also yields the built-in defaults),
//     falling back to the default key files.
//
// Entirely best-effort: any failure just means the clone may prompt or fail,
// which the user sees live. The returned cleanup is never nil.
func ensureAgentHasKey(ctx context.Context, log *stepLogger, cloneURL string) (cleanup func()) {
	cleanup = func() {}

	// Agent forwarding only helps SSH remotes; HTTPS clones don't use it.
	if !isSSHCloneURL(cloneURL) {
		return cleanup
	}
	// ssh-add may prompt for a passphrase, so only attempt it with a real
	// terminal (the rest of rde claude needs one anyway).
	if !cmdutil.IsTerminal(os.Stdin) {
		return cleanup
	}

	switch agentState(ctx) {
	case agentHasKeys, agentUnknown:
		return cleanup
	case agentNoAgent:
		c, err := startAgent(ctx)
		if err != nil {
			log.warn("No SSH agent running and could not start one (%v); git clone of private repos in the session may fail.", err)
			return cleanup
		}
		log.step("Started a temporary SSH agent")
		cleanup = c
		// fall through to add a key to the fresh agent
	case agentEmpty:
		// fall through to add a key
	}

	key := pickIdentityFile(ctx, sshHostFromURL(cloneURL))
	if key == "" {
		log.warn("SSH agent has no keys and no key file was found to add; private clones may fail.")
		return cleanup
	}
	log.step("Adding SSH key %s to the agent…", key)
	add := exec.CommandContext(ctx, "ssh-add", key) //nolint:gosec // G204: key is a local key-file path (from ssh -G or default files), passed as its own argv element — no shell, no injection
	add.Stdin = os.Stdin
	add.Stdout = os.Stderr // ssh-add writes prompts/results to stderr; keep stdout clean
	add.Stderr = os.Stderr
	_ = add.Run() // best-effort; the clone surfaces any remaining auth error
	return cleanup
}

// startAgent launches a temporary `ssh-agent`, exports its socket (and PID)
// into this process's environment so ssh-add and our own agent forwarding
// (dialLocalAgent reads $SSH_AUTH_SOCK) use it, and returns a cleanup that
// terminates the agent. The agent must outlive both the clone and the
// interactive claude session, so the caller defers cleanup to process exit.
func startAgent(ctx context.Context) (func(), error) {
	out, err := exec.CommandContext(ctx, "ssh-agent", "-s").Output()
	if err != nil {
		return nil, err
	}
	sock := parseAgentVar(string(out), "SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("ssh-agent did not report a socket path")
	}
	if err := os.Setenv("SSH_AUTH_SOCK", sock); err != nil {
		return nil, err
	}
	if pid := parseAgentVar(string(out), "SSH_AGENT_PID"); pid != "" {
		_ = os.Setenv("SSH_AGENT_PID", pid)
	}
	return func() {
		// `ssh-agent -k` reads SSH_AGENT_PID/SSH_AUTH_SOCK from the env we just
		// set and kills the agent + removes its socket. Not ctx-bound: ctx is
		// likely already cancelled by the time cleanup runs.
		_ = exec.Command("ssh-agent", "-k").Run()
	}, nil
}

// parseAgentVar extracts NAME's value from `ssh-agent -s` output, whose lines
// look like `SSH_AUTH_SOCK=/tmp/…/agent.123; export SSH_AUTH_SOCK;`.
func parseAgentVar(out, name string) string {
	for line := range strings.SplitSeq(out, "\n") {
		for field := range strings.SplitSeq(line, ";") {
			if rest, ok := strings.CutPrefix(strings.TrimSpace(field), name+"="); ok {
				return rest
			}
		}
	}
	return ""
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
