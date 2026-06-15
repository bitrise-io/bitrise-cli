package rde

import (
	"context"
	"fmt"
	"io"
)

// ExecuteInteractive attaches the caller's terminal to command running on the
// session over SSH and blocks until it exits, returning its exit code. Unlike
// Execute, it allocates a PTY (when stdin is a terminal), runs in raw mode, and
// is NOT capped by ExecuteTimeout — interactive programs are long-lived.
//
// It is the interactive sibling of Execute: same pre-flight checks, same SSH
// dial and agent-forwarding posture, but stdin/stdout/stderr are streamed live
// instead of captured.
func (s *Service) ExecuteInteractive(ctx context.Context, workspaceID, sessionID, command string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if s.client == nil {
		return -1, errClient()
	}
	if command == "" {
		return -1, fmt.Errorf("command is required")
	}
	sess, err := s.GetSession(ctx, workspaceID, sessionID)
	if err != nil {
		return -1, fmt.Errorf("fetch session: %w", err)
	}
	if sess.Status != "running" {
		return -1, fmt.Errorf(
			"session is not running (status: %q); start the session before running commands",
			sess.Status,
		)
	}
	if !sess.SSHConnectionOpen || sess.SSHAddress == "" || sess.SSHPassword == "" {
		return -1, fmt.Errorf(
			"session SSH is not ready yet (credentials not populated); the session may still be provisioning — wait a few seconds and retry",
		)
	}

	target, err := parseSSHAddress(sess.SSHAddress)
	if err != nil {
		return -1, fmt.Errorf("parse session ssh address: %w", err)
	}
	target.Password = sess.SSHPassword

	// Retry the dial: the backend reports SSH ready a moment before the port
	// actually accepts connections, so the first attempts can be refused.
	client, err := dialSSHWithRetry(ctx, target)
	if err != nil {
		return -1, err
	}
	defer client.Close() //nolint:errcheck // run errors take precedence; nothing actionable on close failure

	return client.runInteractive(ctx, command, stdin, stdout, stderr)
}
