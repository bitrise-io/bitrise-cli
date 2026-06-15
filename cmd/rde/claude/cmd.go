// Package claude wires the `bitrise-cli rde claude` command: spin up an
// ephemeral RDE session and attach straight into Claude Code.
package claude

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
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
SSH form so the forwarded agent can authenticate.`,
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

			quiet := cmdutil.IsQuiet(cmd)
			progress := func(format string, a ...any) {
				if quiet {
					return
				}
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), format, a...)
			}

			progress("Creating session %q…\n", name)
			res, err := svc.CreateSession(ctx, workspaceID, internalrde.CreateSessionRequest{
				Name:       name,
				TemplateID: templateID,
			})
			if err != nil {
				return err
			}
			sessionID := res.Session.ID

			// Single-use session: tear it down on every exit path, including
			// PTY errors and Ctrl-C. A fresh context is used because
			// cmd.Context() may already be cancelled by the time we get here.
			defer func() {
				progress("Terminating session %q…\n", name)
				termCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if _, termErr := svc.TerminateSession(termCtx, workspaceID, sessionID); termErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to terminate session %q: %v\n", sessionID, termErr)
				}
			}()

			progress("Waiting for session to start (timeout %s)…\n", waitTimeout)
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
			progress("Waiting for SSH access…\n")
			if _, err := svc.WaitForSSHReady(waitCtx, workspaceID, sessionID, 0); err != nil {
				return fmt.Errorf("waiting for SSH access: %w", err)
			}

			// The clone authenticates via the forwarded local SSH agent; make
			// sure it has a key loaded before we dial in for the clone.
			ensureAgentHasKey(ctx, progress, cloneURL)

			// Clone the same repo + branch into the session over the
			// forwarded SSH agent. Runs in its own interactive session so its
			// output (git progress) streams live with no timeout.
			// StrictHostKeyChecking=accept-new avoids a host-key prompt that
			// would otherwise hang the non-interactive clone.
			progress("Cloning %s (branch %s) into the session…\n", repoDir, branch)
			cloneCmd := fmt.Sprintf("GIT_SSH_COMMAND=%s git clone --branch %s %s %s",
				cmdutil.ShellQuote("ssh -o StrictHostKeyChecking=accept-new"),
				cmdutil.ShellQuote(branch),
				cmdutil.ShellQuote(cloneURL),
				cmdutil.ShellQuote(repoDir),
			)
			cloneCode, err := svc.ExecuteInteractive(ctx, workspaceID, sessionID, cloneCmd, os.Stdin, os.Stdout, os.Stderr)
			if err != nil {
				return err
			}
			if cloneCode != 0 {
				cmdutil.SilenceRootErrors(cmd)
				return fmt.Errorf("git clone failed (status %d)", cloneCode)
			}

			progress("Connecting…\n")
			// cd into the clone, then `exec claude` so the login-interactive
			// bash replaces itself with Claude Code — the user lands directly
			// in claude (inside the repo), not a shell.
			claudeCmd := "cd " + cmdutil.ShellQuote(repoDir) + " && exec claude"
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
