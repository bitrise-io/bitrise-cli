package rde

import (
	"context"
	"fmt"
	"time"
)

// ExecuteTimeout caps how long an individual `rde session exec` invocation
// is allowed to run. Matches the MCP's 2-minute ceiling — long-running
// jobs should run nohup'd in the session itself, not under exec.
const ExecuteTimeout = 2 * time.Minute

// Execute runs command on the session via SSH and returns its captured
// stdout/stderr/exit_code. Mirrors the MCP's `bitrise_devenv_execute`
// behavior: forced-interactive login bash (`bash -i -l -c`), local
// SSH agent forwarded so git-over-SSH uses the caller's keys.
//
// Errors fall into three categories:
//   - "session not running" / "ssh not ready" — surfaced before the dial
//   - dial/handshake/network failures — surfaced as errors
//   - command exited non-zero — returned in ExecResult with a nil error;
//     callers decide how to surface that to the user
func (s *Service) Execute(ctx context.Context, workspaceID, sessionID, command string) (ExecResult, error) {
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

	execCtx, cancel := context.WithTimeout(ctx, ExecuteTimeout)
	defer cancel()

	client, err := dialSSH(execCtx, target)
	if err != nil {
		return ExecResult{}, err
	}
	defer client.Close() //nolint:errcheck // run errors take precedence; nothing actionable on close failure

	return client.run(execCtx, command)
}
