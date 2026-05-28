package session

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newExecCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "exec SESSION_ID -- COMMAND [ARGS...]",
		Short: "Run a bash command on a session over SSH",
		Long: `Run a bash command on a session over SSH and capture its output.

The command runs in a forced-interactive login bash shell (bash -i -l -c) so
the session's PATH, brew-installed binaries, git-lfs, and language version
managers (nvm, pyenv, rbenv, asdf) are all loaded.

If a local SSH agent is available ($SSH_AUTH_SOCK set), it's forwarded into
the session — git-over-SSH inside the session uses the caller's local keys.

In human mode: stdout streams to this CLI's stdout, stderr to stderr, and
this CLI exits non-zero when the remote command exits non-zero.

In --output json mode: a single {"exit_code", "stdout", "stderr"} object is
emitted to stdout regardless of the command's exit status.

NOTE: because the remote shell is forced-interactive without a TTY, bash
emits two harmless startup diagnostic lines to stderr on every invocation
("cannot set terminal process group", "no job control in this shell").
Ignore them — the exit code is the source of truth.

The remote command is capped at 2 minutes. For long-running jobs, nohup
them inside the session.`,
		Example: `  bitrise-cli rde session exec SESSION_ID -- echo hello
  bitrise-cli rde session exec SESSION_ID -- npm test
  bitrise-cli rde session exec SESSION_ID --output json -- 'ls -la /opt'`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("session exec requires SESSION_ID followed by a command (e.g. 'rde session exec ID -- echo hi')")
			}
			return nil
		},
		// DisableFlagParsing keeps cobra from consuming flag-like tokens
		// inside the user's command. Combined with the explicit `--`
		// convention, it lets users write e.g. `exec ID -- ls -la /opt`
		// without the `-la` being interpreted as a flag of our command.
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]
			command := joinShellArgs(args[1:])
			if strings.TrimSpace(command) == "" {
				return fmt.Errorf("command is empty")
			}
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			res, err := internalrde.NewService(client).Execute(cmd.Context(), workspaceID, sessionID, command)
			if err != nil {
				return err
			}
			return renderExecResult(cmd, format, res)
		},
	}
	return c
}

// joinShellArgs joins argv into a single shell-safe command string by
// POSIX-quoting each argument. Without quoting, `bash -c 'echo a; pwd'`
// would arrive at the remote as three space-separated tokens and the `;`
// would be reinterpreted by the remote shell, dropping `echo a`.
func joinShellArgs(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		q[i] = shellQuote(a)
	}
	return strings.Join(q, " ")
}

// shellQuote wraps s in POSIX single-quotes, escaping any embedded single
// quotes with the standard `'\”` sequence. Strings made entirely of
// shell-safe characters are returned as-is to keep readable commands
// readable in logs.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '_', c == '-', c == '.', c == '/', c == ':', c == '=', c == ',', c == '@', c == '+', c == '%':
		default:
			safe = false
		}
		if !safe {
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func renderExecResult(cmd *cobra.Command, format output.Format, res internalrde.ExecResult) error {
	if format == output.JSON {
		return output.Render(cmd.OutOrStdout(), format, res, nil)
	}
	// Human mode: stream stdout and stderr to their natural sinks. We
	// don't echo a JSON-shaped envelope — users piping `| jq` should be
	// using --output json.
	if res.Stdout != "" {
		if _, err := io.WriteString(cmd.OutOrStdout(), res.Stdout); err != nil {
			return err
		}
	}
	if res.Stderr != "" {
		if _, err := io.WriteString(cmd.ErrOrStderr(), res.Stderr); err != nil {
			return err
		}
	}
	if res.ExitCode != 0 {
		// Suppress cobra's "Error: ..." line — the remote command's
		// stderr already explains the failure.
		cmdutil.SilenceRootErrors(cmd)
		return fmt.Errorf("remote command exited with status %d", res.ExitCode)
	}
	return nil
}
