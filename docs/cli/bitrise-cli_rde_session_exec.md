## bitrise-cli rde session exec

Run a command on a session over SSH

### Synopsis

Run a command on a session over SSH and capture its output.

The command runs in a forced-interactive login bash shell (bash -i -l -c) so
the session's PATH, brew-installed binaries, git-lfs, and language version
managers (nvm, pyenv, rbenv, asdf) are all loaded.

By default the tokens after '--' are treated as a program plus literal
arguments: each is passed through verbatim, so shell metacharacters (;, &&,
|, $(...), redirection) are NOT interpreted — 'exec ID -- echo a; b' runs
echo with the literal argument 'a; b'. This keeps quoted arguments intact
(e.g. -m "a message").

Pass --shell to interpret everything after '--' as a shell command line
instead, so pipes, &&, command substitution and redirection work:

  bitrise-cli rde session exec SESSION_ID --shell -- 'cd repo && xcodebuild | xcpretty'

This is the first-class replacement for hand-wrapping every command in
bash -lc "…". Quote the command (or its metacharacters) so your local shell
passes them through unchanged.

If a local SSH agent is available ($SSH_AUTH_SOCK set), it's forwarded into
the session — git-over-SSH inside the session uses the caller's local keys.

In human mode: stdout streams to this CLI's stdout, stderr to stderr, and
this CLI exits non-zero when the remote command exits non-zero.

In --output json mode: a single {"exit_code", "stdout", "stderr"} object is
emitted to stdout regardless of the command's exit status.

The remote command is capped at 2 minutes. For long-running jobs, nohup
them inside the session.

```
bitrise-cli rde session exec SESSION_ID -- COMMAND [ARGS...] [flags]
```

### Examples

```
  bitrise-cli rde session exec SESSION_ID -- echo hello
  bitrise-cli rde session exec SESSION_ID -- npm test
  bitrise-cli rde session exec SESSION_ID --shell -- 'cd repo && ls | head'
  bitrise-cli rde session exec SESSION_ID --output json -- ls -la /opt
```

### Options

```
  -h, --help    help for exec
      --shell   interpret everything after '--' as a shell command line (pipes, &&, $(...), redirection) instead of a program with literal arguments
```

### Options inherited from parent commands

```
      --no-color           disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string      output format: human|json (default "human")
  -q, --quiet              suppress non-error diagnostic messages
      --theme string       color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
      --workspace string   workspace ID (or set BITRISE_WORKSPACE_ID or default_workspace_id; auto-detected if you have exactly one workspace)
```

### SEE ALSO

* [bitrise-cli rde session](bitrise-cli_rde_session.md)	 - Create, list, inspect, and manage RDE sessions

