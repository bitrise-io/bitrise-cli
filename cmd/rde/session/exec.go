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
	var shellMode bool
	c := &cobra.Command{
		Use:   "exec SESSION_ID -- COMMAND [ARGS...]",
		Short: "Run a command on a session over SSH",
		Long: `Run a command on a session over SSH and capture its output.

The command runs in a forced-interactive login bash shell (bash -i -l -c) so
the session's PATH, brew-installed binaries, git-lfs, and language version
managers (nvm, pyenv, rbenv, asdf) are all loaded.

By default the tokens after '--' are treated as a program plus literal
arguments: each is passed through verbatim, so shell metacharacters (;, &&,
|, $(...), redirection) are NOT interpreted — 'exec ID -- echo "a; b"' runs
echo with the single literal argument 'a; b'. This keeps quoted arguments
intact (e.g. -m "a message"). (Quote such arguments so your local shell
hands them over as one token rather than splitting them itself.)

Pass --shell to interpret everything after '--' as a shell command line
instead, so pipes, &&, command substitution and redirection work:

  bitrise-cli rde session exec SESSION_ID --shell -- 'cd repo && xcodebuild | xcpretty'

This is the first-class replacement for hand-wrapping every command in
bash -lc "…". Quote the command (or its metacharacters) so your local shell
passes them through unchanged. --shell must come before '--'; after '--' it
is just another literal token.

If a local SSH agent is available ($SSH_AUTH_SOCK set), it's forwarded into
the session — git-over-SSH inside the session uses the caller's local keys.

In human mode: stdout streams to this CLI's stdout, stderr to stderr, and
this CLI exits non-zero when the remote command exits non-zero.

In --output json mode: a single {"exit_code", "stdout", "stderr"} object is
emitted to stdout regardless of the command's exit status.

The remote command is capped at 2 minutes. For long-running jobs, nohup
them inside the session.`,
		Example: `  bitrise-cli rde session exec SESSION_ID -- echo hello
  bitrise-cli rde session exec SESSION_ID -- npm test
  bitrise-cli rde session exec SESSION_ID -- git commit -m "a message"
  bitrise-cli rde session exec SESSION_ID --shell -- 'cd repo && ls | head'
  bitrise-cli rde session exec SESSION_ID --output json -- ls -la /opt`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("session exec requires SESSION_ID followed by a command (e.g. 'rde session exec ID -- echo hi')")
			}
			return nil
		},
		// DisableFlagParsing stays off so our own flags (--shell, --output …)
		// are parsed. Combined with the explicit `--` convention, tokens after
		// `--` are still handed through as positional args, so a user can write
		// `exec ID -- ls -la /opt` without `-la` being read as our flag.
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]
			command := buildExecCommand(args[1:], shellMode)
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
			svc := internalrde.NewService(client)
			sessionID, err = svc.ResolveSessionID(cmd.Context(), workspaceID, sessionID)
			if err != nil {
				return err
			}
			res, err := svc.Execute(cmd.Context(), workspaceID, sessionID, command)
			if err != nil {
				return err
			}
			return renderExecResult(cmd, format, res)
		},
	}
	c.Flags().BoolVar(&shellMode, "shell", false, "interpret everything after '--' as a shell command line (pipes, &&, $(...), redirection) instead of a program with literal arguments")
	return c
}

// buildExecCommand turns the post-`--` tokens into the command string sent to
// the session. The remote side always wraps it in `bash -i -l -c '<cmd>'`
// (see internal/rde), so the only question here is how the tokens are joined:
//
//   - shell mode: joined verbatim with spaces, so the wrapped bash interprets
//     metacharacters (matches the MCP's single `command` string and the
//     backend's `bash_c_command`). The caller is responsible for quoting so
//     their local shell passes the metacharacters through.
//   - default: each token is POSIX-quoted, so it reaches bash as a literal
//     argv element and shell metacharacters are inert.
func buildExecCommand(cmdArgs []string, shell bool) string {
	if shell {
		return strings.Join(cmdArgs, " ")
	}
	return cmdutil.JoinShellArgs(cmdArgs)
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
