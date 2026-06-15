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
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalapp "github.com/bitrise-io/bitrise-cli/internal/app"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

// templateName is the RDE template this command always provisions from.
const templateName = "szabi linux empty"

// NewCmd returns the `bitrise-cli rde claude` command.
func NewCmd() *cobra.Command {
	var waitTimeout time.Duration

	c := &cobra.Command{
		Use:   "claude",
		Short: "Create an ephemeral RDE session and attach to Claude Code",
		Long: `Create a fresh RDE session from the "` + templateName + `" template, wait for it
to start, then SSH in and drop you directly into Claude Code (not a shell).

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

Unless a Claude Code token is already configured on the control plane, your
local Claude Code credentials are forwarded so the in-session claude is logged
in. The credential is taken from $CLAUDE_CODE_OAUTH_TOKEN or $ANTHROPIC_API_KEY,
then ~/.claude/.credentials.json; if none is found, 'claude setup-token' is run
to mint one (browser auth) and the result is saved on the control plane so
future sessions don't need to mint again.`,
		Example: `  bitrise-cli rde claude --workspace WORKSPACE_ID`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

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

			templateID, err := svc.ResolveTemplateID(ctx, workspaceID, templateName)
			if err != nil {
				return err
			}

			name, err := generateSessionName()
			if err != nil {
				return err
			}

			log := newStepLogger(cmd)

			// ── Session ────────────────────────────────────────────────────
			log.group("Session")

			// If the control plane already has a Claude Code token (saved
			// input), map it into the session and skip forwarding the local
			// one. Best-effort: on a failed lookup, treat as absent and forward.
			controlPlaneToken, cpErr := controlPlaneHasClaudeToken(ctx, svc)
			if cpErr != nil {
				controlPlaneToken = false
			}

			log.step("Creating session %q…", name)
			res, err := svc.CreateSession(ctx, workspaceID, internalrde.CreateSessionRequest{
				Name:                    name,
				TemplateID:              templateID,
				MapSavedToSessionInputs: controlPlaneToken,
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
			cloneCode, err := svc.ExecuteInteractive(ctx, workspaceID, sessionID, cloneCmd, os.Stdin, os.Stdout, os.Stderr)
			if err != nil {
				return err
			}
			if cloneCode != 0 {
				cmdutil.SilenceRootErrors(cmd)
				return fmt.Errorf("git clone failed (status %d)", cloneCode)
			}
			log.done("Repository cloned")

			// ── Claude Code ────────────────────────────────────────────────
			log.group("Claude Code")

			// Forward local Claude Code auth so the in-session claude is logged
			// in — unless the control plane already provides a token.
			var claudeEnvVar, claudeEnvVal string
			if controlPlaneToken {
				log.step("Auth: control plane token")
			} else {
				cred, ok := existingLocalCredential()
				if !ok {
					// Nothing local — mint a long-lived token (interactive browser auth).
					log.step("No local credentials; minting one with 'claude setup-token'…")
					cred, ok = mintSetupToken(ctx)
				}
				if ok {
					claudeEnvVar, claudeEnvVal = cred.EnvVar, cred.Value
					log.step("Auth: forwarded %s (from %s)", cred.EnvVar, cred.Source)
					// Persist durable creds (env token/key or a freshly minted
					// token) on the control plane so future sessions pick them
					// up without re-forwarding.
					if cred.Persist {
						if _, saveErr := svc.CreateSavedInput(ctx, internalrde.CreateSavedInputRequest{
							Key: cred.EnvVar, Value: cred.Value, IsSecret: true,
						}); saveErr != nil {
							log.warn("Failed to save the token on the control plane (%v); it won't persist for next time.", saveErr)
						} else {
							log.step("Saved the token on the control plane for future sessions")
						}
					}
				} else {
					log.warn("Auth: none found — you may need to log in inside the session.")
				}
			}

			log.step("Starting…")
			// cd into the clone, then `exec claude` so the login-interactive
			// bash replaces itself with Claude Code — the user lands directly
			// in claude (inside the repo), not a shell.
			claudeCmd := buildClaudeCommand(repoDir, claudeEnvVar, claudeEnvVal)
			exitCode, err := svc.ExecuteInteractive(ctx, workspaceID, sessionID, claudeCmd, os.Stdin, os.Stdout, os.Stderr)
			if err != nil {
				return err
			}
			if exitCode != 0 {
				cmdutil.SilenceRootErrors(cmd)
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
// inside repoDir. When envVar is non-empty, it exports the forwarded credential
// first so the in-session claude is authenticated. The value is shell-quoted;
// the var name is a fixed identifier. `exec` replaces the login-interactive
// bash with claude, so the token drops out of the VM's process list.
func buildClaudeCommand(repoDir, envVar, envVal string) string {
	cd := "cd " + cmdutil.ShellQuote(repoDir) + " && exec claude"
	if envVar == "" {
		return cd
	}
	return "export " + envVar + "=" + cmdutil.ShellQuote(envVal) + " && " + cd
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
