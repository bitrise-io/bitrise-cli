// Package claude wires the `bitrise-cli rde claude` command: spin up an
// ephemeral RDE session and attach straight into Claude Code.
package claude

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalapp "github.com/bitrise-io/bitrise-cli/internal/app"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

// The command provisions a templateless session from a fixed image + machine
// type, so there's no template to resolve or keep in sync.
const (
	sessionImage       = "linux-bitvirt-2026"
	sessionMachineType = "g2.linux.amd-zen5.8c-32g"
)

// NewCmd returns the `bitrise-cli rde claude` command.
func NewCmd() *cobra.Command {
	var waitTimeout time.Duration

	c := &cobra.Command{
		Use:   "claude",
		Short: "Create an ephemeral RDE session and attach to Claude Code",
		Long: `Create a fresh RDE session on the "` + sessionImage + `" image, wait for it to
start, then SSH in and drop you directly into Claude Code (not a shell).

Run this from inside a git repository: the session clones the same repository
and branch you're on (via 'git clone') and starts Claude Code inside that
clone. Only the pushed remote state of the branch is cloned — local
uncommitted or unpushed changes are not transferred.

The session is single-use: when you exit Claude Code, the session is
terminated automatically. Each invocation creates a new, uniquely named
session (claude-<id>).

A local SSH agent ($SSH_AUTH_SOCK), if present, is forwarded into the session
so the clone (and git-over-SSH inside the session) uses your local keys. If the
repo's origin is an HTTPS GitHub/GitLab/Bitbucket URL, it's rewritten to its
SSH form so the forwarded agent can authenticate.

Unless a Claude Code token is already configured on the control plane, a local
credential is saved there before the session is created — taken from
$CLAUDE_CODE_OAUTH_TOKEN or $ANTHROPIC_API_KEY, then ~/.claude/.credentials.json,
or minted with 'claude setup-token' (browser auth). The control plane uses that
token to install Claude Code and tmux during provisioning and to authenticate
the in-session claude; once saved, future sessions reuse it.`,
		Example: `  bitrise-cli rde claude --workspace WORKSPACE_ID`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Make the command interrupt-aware: Ctrl-C cancels ctx, which kills
			// any child we launched (e.g. 'claude setup-token') and lets the
			// deferred session cleanup run instead of hard-killing the process.
			// (During the interactive Claude attach the terminal is in raw mode,
			// so Ctrl-C is delivered to the remote claude, not to us — as
			// intended.)
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()

			// Resolve the local repo + branch FIRST so a non-repo cwd fails
			// before we create (and would have to tear down) a session.
			detector := internalapp.ExecGitDetector{}
			originURL, err := detector.RemoteURL(ctx)
			if err != nil {
				return fmt.Errorf("detect git remote: %w", err)
			}
			if originURL == "" {
				return fmt.Errorf("current directory is not a git repository (or has no 'origin' remote); rde claude clones the current repo into the session")
			}
			branch, err := detector.CurrentBranch(ctx)
			if err != nil {
				return fmt.Errorf("detect git branch: %w", err)
			}
			if branch == "" {
				return fmt.Errorf("could not determine the current git branch (detached HEAD?); check out a branch before running rde claude")
			}
			cloneURL := gitSSHURL(originURL)
			repoDir := repoDirFromURL(originURL)

			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			svc := internalrde.NewService(client)

			name, err := generateSessionName()
			if err != nil {
				return err
			}

			log := newStepLogger(cmd)

			// ── Claude Code auth ───────────────────────────────────────────
			// Ensure a Claude Code token exists on the control plane *before*
			// creating the session: the backend installs claude + tmux during
			// provisioning only when the token saved input is present, and maps
			// it into the session as an env var so claude is authenticated.
			// Without a token the session would be useless for this command, so
			// a failure here aborts before any VM is created.
			log.group("Claude Code auth")
			if err := ensureClaudeAuth(ctx, svc, log); err != nil {
				return err
			}

			// ── Session ────────────────────────────────────────────────────
			log.group("Session")
			log.step("Creating session %q…", name)
			res, err := svc.CreateSession(ctx, workspaceID, internalrde.CreateSessionRequest{
				Name:        name,
				Image:       sessionImage,
				MachineType: sessionMachineType,
				// Auth is guaranteed present by now, so map the token saved
				// input in: provisioning installs claude + tmux and the session
				// is authenticated.
				MapSavedToSessionInputs: true,
			})
			if err != nil {
				return err
			}
			sessionID := res.Session.ID

			// Single-use session: tear it down on every exit path, including
			// PTY errors and Ctrl-C. A fresh context is used because
			// cmd.Context() may already be cancelled by the time we get here.
			defer func() {
				log.group("Cleanup")
				log.step("Terminating session %q…", name)
				termCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if _, termErr := svc.TerminateSession(termCtx, workspaceID, sessionID); termErr != nil {
					log.warn("Failed to terminate session %q: %v", sessionID, termErr)
					return
				}
				log.done("Session terminated")
			}()

			log.step("Waiting for it to start (timeout %s)…", waitTimeout)
			// The whole startup wait — reaching "running" and then SSH
			// credentials being issued — shares one timeout budget.
			waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
			defer cancel()

			ready, err := svc.WaitForReady(waitCtx, workspaceID, sessionID, 0)
			if err != nil {
				return fmt.Errorf("waiting for session: %w", err)
			}
			if ready.Status != "running" {
				cmdutil.SilenceRootErrors(cmd)
				return fmt.Errorf("session ended provisioning with status %q (expected running)", ready.Status)
			}

			// "running" does not mean SSH is reachable yet — the backend
			// issues credentials a few seconds later. Wait for them before
			// dialing in.
			log.step("Waiting for remote access…")
			if _, err := svc.WaitForSSHReady(waitCtx, workspaceID, sessionID, 0); err != nil {
				return fmt.Errorf("waiting for SSH access: %w", err)
			}
			log.done("Session ready")

			// ── Repository ─────────────────────────────────────────────────
			log.group("Repository")

			// The clone authenticates via the forwarded local SSH agent; make
			// sure an agent is running with a key loaded before we dial in.
			// cleanupAgent kills a temporary agent we may have started; it
			// must outlive the claude session, so defer it to command exit.
			cleanupAgent, repoAuth := ensureAgentHasKey(ctx, log, cloneURL)
			defer cleanupAgent()
			log.step("Auth: %s", repoAuth)

			// Clone the same repo + branch into the session over the
			// forwarded SSH agent. Runs in its own interactive session so its
			// output (git progress) streams live with no timeout.
			// StrictHostKeyChecking=accept-new avoids a host-key prompt that
			// would otherwise hang the non-interactive clone.
			//
			// If the current branch hasn't been pushed (not on the remote),
			// fall back to cloning the remote's default branch.
			useDefaultBranch := false
			if found, determined := remoteHasBranch(ctx, branch); determined && !found {
				useDefaultBranch = true
			}
			if useDefaultBranch {
				log.warn("Branch %q is not on the remote; cloning the default branch instead.", branch)
				log.step("Cloning %s (default branch)…", repoDir)
			} else {
				log.step("Cloning %s (branch %s)…", repoDir, branch)
			}
			cloneCmd := buildCloneCommand(cloneURL, repoDir, branch, useDefaultBranch)
			// Indent the clone's streamed output so it lines up under the
			// group. (The interactive Claude attach below streams raw — a
			// full-screen TUI can't be column-shifted.)
			cloneCode, err := svc.ExecuteInteractive(ctx, workspaceID, sessionID, cloneCmd, os.Stdin, newIndentWriter(os.Stdout), newIndentWriter(os.Stderr))
			if err != nil {
				return err
			}
			if cloneCode != 0 {
				cmdutil.SilenceRootErrors(cmd)
				return fmt.Errorf("git clone failed (status %d)", cloneCode)
			}
			log.done("Repository cloned")

			// ── Claude Code ────────────────────────────────────────────────
			// Auth was provisioned up front (token saved input → session env
			// var), so just start claude — no credential injection here.
			log.group("Claude Code")
			log.step("Starting…")
			// Start claude inside a tmux session (in the cloned repo) so it
			// survives a disconnect and can be reattached later; the user still
			// lands directly in claude, not a shell.
			claudeCmd := buildClaudeCommand(repoDir)
			exitCode, err := svc.ExecuteInteractive(ctx, workspaceID, sessionID, claudeCmd, os.Stdin, os.Stdout, os.Stderr)
			if errors.Is(err, internalrde.ErrConnectionLost) {
				// The connection dropped but claude keeps running in tmux on
				// the session — reattach instead of tearing it down.
				log.group("Reconnecting")
				log.warn("Connection lost; Claude Code is still running in tmux on the session.")
				exitCode, err = reattachWithRetry(ctx, svc, log, workspaceID, sessionID)
			}
			if err != nil {
				return err
			}
			if exitCode != 0 {
				cmdutil.SilenceRootErrors(cmd)
				if exitCode == 127 {
					// 127 = "command not found" from the login shell.
					return fmt.Errorf("could not start Claude Code: 'tmux' or 'claude' is not installed on the session")
				}
				return fmt.Errorf("claude exited with status %d", exitCode)
			}
			return nil
		},
	}

	c.Flags().DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "max time to wait for the session to start (uses Go duration syntax: 30s, 5m, 1h)")
	return c
}

// generateSessionName returns a unique "claude-<hex>" session name. There is
// no UUID dependency in this module, so a short random suffix from crypto/rand
// is used to keep names collision-free across invocations.
func generateSessionName() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session name: %w", err)
	}
	return "claude-" + hex.EncodeToString(b), nil
}

// remoteHasBranch reports whether branch exists on the cwd repo's "origin"
// remote. `git ls-remote --exit-code` exits 0 when the ref is found and 2 when
// it isn't; any other outcome (network/auth error, git missing) leaves it
// undetermined, in which case the caller keeps the requested branch.
func remoteHasBranch(ctx context.Context, branch string) (found, determined bool) {
	//nolint:gosec // G204: branch comes from the local repo's checked-out HEAD, passed as its own argv element — no shell, no injection
	err := exec.CommandContext(ctx, "git", "ls-remote", "--heads", "--exit-code", "origin", branch).Run()
	if err == nil {
		return true, true
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 2 {
		return false, true
	}
	return false, false
}

// buildCloneCommand returns the remote shell command that clones cloneURL into
// repoDir. When useDefaultBranch is true it omits --branch, so git clones the
// remote's default branch (the fallback when the local branch isn't pushed).
// GIT_SSH_COMMAND's accept-new keeps the non-interactive clone from hanging on
// a host-key prompt.
func buildCloneCommand(cloneURL, repoDir, branch string, useDefaultBranch bool) string {
	sshEnv := "GIT_SSH_COMMAND=" + cmdutil.ShellQuote("ssh -o StrictHostKeyChecking=accept-new")
	if useDefaultBranch {
		return fmt.Sprintf("%s git clone %s %s",
			sshEnv, cmdutil.ShellQuote(cloneURL), cmdutil.ShellQuote(repoDir))
	}
	return fmt.Sprintf("%s git clone --branch %s %s %s",
		sshEnv, cmdutil.ShellQuote(branch), cmdutil.ShellQuote(cloneURL), cmdutil.ShellQuote(repoDir))
}

// buildClaudeCommand returns the remote shell command that starts Claude Code
// inside repoDir, running it in a tmux session so it survives an SSH
// disconnect and can be reattached later. Auth comes from the session's
// environment (the control-plane token saved input mapped in as an env var),
// so nothing is injected here.
//
//   - `tmux new-session -A -s claude`: attach to the "claude" session if it
//     already exists, else create it.
//   - `-c <repoDir>`: the pane starts in the cloned repo.
//   - `exec claude`: the pane's shell becomes claude, so the tmux session ends
//     when claude exits.
func buildClaudeCommand(repoDir string) string {
	return "tmux new-session -A -s claude -c " + cmdutil.ShellQuote(repoDir) + " " + cmdutil.ShellQuote("exec claude")
}

// buildReattachCommand reattaches to the running "claude" tmux session after a
// dropped connection. It does NOT recreate the session: if claude already
// exited, tmux exits non-zero and we stop rather than starting claude over.
func buildReattachCommand() string {
	return "tmux attach-session -t claude"
}

// ensureClaudeAuth makes sure a Claude Code token is configured on the control
// plane (a CLAUDE_CODE_OAUTH_TOKEN / ANTHROPIC_API_KEY saved input). When it's
// missing, it resolves a local credential — env var, ~/.claude/.credentials.json,
// or a freshly minted `claude setup-token` (browser auth) — and saves it.
//
// A token is mandatory: it's what makes the backend install claude + tmux and
// authenticate the in-session claude, so the session is useless without one.
// Any failure (control plane unreachable, no credential found, setup-token
// cancelled/declined, save failed, or the run interrupted) returns an error so
// the caller aborts before creating a VM.
func ensureClaudeAuth(ctx context.Context, svc *internalrde.Service, log *stepLogger) error {
	has, err := controlPlaneHasClaudeToken(ctx, svc)
	if err != nil {
		// We can't tell whether a token is configured. The same API is needed
		// to save the token and create the session, so abort with a clear
		// message rather than detouring through an unexpected setup-token flow.
		return fmt.Errorf("check Claude Code token on the control plane: %w", err)
	}
	if has {
		log.step("Using the Claude Code token already on the control plane")
		return nil
	}

	cred, ok := existingLocalCredential()
	if !ok {
		log.step("No token on the control plane or locally; minting one with 'claude setup-token'…")
		cred, ok = mintSetupToken(ctx)
	}
	if !ok {
		// A cancelled/interrupted run surfaces as the context error so the
		// user sees "interrupted" rather than a misleading "no token" message.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("no Claude Code token available ('claude setup-token' did not return one); not creating a session. " +
			"Set $CLAUDE_CODE_OAUTH_TOKEN or $ANTHROPIC_API_KEY, log in locally, or configure a token on the control plane, then retry")
	}

	if _, err := svc.CreateSavedInput(ctx, internalrde.CreateSavedInputRequest{
		Key: cred.EnvVar, Value: cred.Value, IsSecret: true,
	}); err != nil {
		return fmt.Errorf("save Claude Code token on the control plane: %w", err)
	}
	log.step("Saved %s on the control plane (from %s)", cred.EnvVar, cred.Source)
	return nil
}

const (
	reconnectInterval = 3 * time.Second
	reconnectTimeout  = 5 * time.Minute
)

// reattachWithRetry keeps reattaching to the session's tmux after the
// connection drops, until claude exits cleanly, the retry budget elapses, or
// the context is cancelled. Each attempt's dial is itself retried inside
// ExecuteInteractive, so a brief outage resolves on the next attempt.
func reattachWithRetry(ctx context.Context, svc *internalrde.Service, log *stepLogger, workspaceID, sessionID string) (int, error) {
	deadline := time.Now().Add(reconnectTimeout)
	for attempt := 1; ; attempt++ {
		log.step("Reattaching to Claude Code (attempt %d)…", attempt)
		code, err := svc.ExecuteInteractive(ctx, workspaceID, sessionID, buildReattachCommand(), os.Stdin, os.Stdout, os.Stderr)
		if !errors.Is(err, internalrde.ErrConnectionLost) {
			return code, err // clean exit, real exit code, or a fatal error
		}
		if time.Now().After(deadline) {
			return -1, fmt.Errorf("gave up reconnecting after %s: %w", reconnectTimeout, err)
		}
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		case <-time.After(reconnectInterval):
		}
	}
}

// sshRewriteHosts are the hosts whose HTTPS clone URLs we rewrite to SSH form,
// so the forwarded SSH agent can authenticate the in-session clone. Other
// hosts are left untouched (their HTTPS URLs only work for public repos).
var sshRewriteHosts = map[string]bool{
	"github.com":    true,
	"gitlab.com":    true,
	"bitbucket.org": true,
}

// gitSSHURL rewrites an HTTPS clone URL for a known host into its SSH form
// (https://github.com/org/repo(.git) → git@github.com:org/repo.git) so the
// forwarded agent's keys authenticate. URLs that are already SSH (git@… or
// ssh://…), or HTTPS for an unknown host, are returned unchanged.
func gitSSHURL(raw string) string {
	if strings.HasPrefix(raw, "git@") || strings.HasPrefix(raw, "ssh://") {
		return raw
	}
	const httpsPrefix = "https://"
	if !strings.HasPrefix(raw, httpsPrefix) {
		return raw
	}
	rest := strings.TrimPrefix(raw, httpsPrefix)
	host, path, ok := strings.Cut(rest, "/")
	if !ok || path == "" {
		return raw
	}
	// Strip any "user@" credential prefix on the host (e.g. user@github.com).
	if _, h, found := strings.Cut(host, "@"); found {
		host = h
	}
	if !sshRewriteHosts[host] {
		return raw
	}
	path = strings.TrimSuffix(path, "/")
	if !strings.HasSuffix(path, ".git") {
		path += ".git"
	}
	return "git@" + host + ":" + path
}

// repoDirFromURL derives the directory name `git clone` would create from a
// clone URL: the last path segment with any ".git" suffix removed (e.g.
// git@github.com:org/repo.git → repo, https://host/org/repo → repo).
func repoDirFromURL(raw string) string {
	s := strings.TrimSuffix(raw, ".git")
	s = strings.TrimRight(s, "/")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	return s
}
