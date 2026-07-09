package rde

import (
	"context"
	"fmt"
	"time"
)

// DefaultExecuteTimeout is the default cap on a single `rde session exec`
// invocation. exec runs client-side over SSH, so this ceiling is the CLI's
// own — not a backend limit — and callers override it (including disabling it
// with 0) via the --timeout flag. It's set generously so ordinary build steps
// (clone, LFS hydration, xcodegen, a warm xcodebuild) finish under one exec;
// the genuinely long cold builds pass a larger --timeout or 0 to uncap.
const DefaultExecuteTimeout = 10 * time.Minute

// Execute runs command on the session via SSH and returns its captured
// stdout/stderr/exit_code. Mirrors the MCP's `bitrise_devenv_execute`
// behavior: forced-interactive login bash (`bash -i -l -c`), local
// SSH agent forwarded so git-over-SSH uses the caller's keys.
//
// timeout caps the whole dial+run. A non-positive timeout disables the cap,
// leaving the run bounded only by ctx and the SSH keepalive that tears the
// connection down within ~10s if it drops (so a genuinely dead connection
// still fails fast even when uncapped).
//
// Errors fall into three categories:
//   - "session not running" / "ssh not ready" — surfaced before the dial
//   - dial/handshake/network failures — surfaced as errors
//   - command exited non-zero — returned in ExecResult with a nil error;
//     callers decide how to surface that to the user
func (s *Service) Execute(ctx context.Context, workspaceID, sessionID, command string, timeout time.Duration) (ExecResult, error) {
	if s.client == nil {
		return ExecResult{}, errClient()
	}
	if command == "" {
		return ExecResult{}, fmt.Errorf("command is required")
	}
	sess, err := s.GetSession(ctx, workspaceID, sessionID)
	if err != nil {
		return ExecResult{}, fmt.Errorf("fetch session: %w", err)
	}
	target, err := sshTargetForSession(sess)
	if err != nil {
		return ExecResult{}, err
	}

	execCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	client, err := dialSSH(execCtx, target)
	if err != nil {
		return ExecResult{}, err
	}
	defer client.Close() //nolint:errcheck // run errors take precedence; nothing actionable on close failure

	return client.run(execCtx, command)
}
