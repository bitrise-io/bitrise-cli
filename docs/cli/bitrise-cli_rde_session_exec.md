## bitrise-cli rde session exec

Run a bash command on a session over SSH

### Synopsis

Run a bash command on a session over SSH and capture its output.

The command runs in a forced-interactive login bash shell (bash -i -l -c) so
the session's PATH, brew-installed binaries, git-lfs, and language version
managers (nvm, pyenv, rbenv, asdf) are all loaded.

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
  bitrise-cli rde session exec SESSION_ID --output json -- 'ls -la /opt'
```

### Options

```
  -h, --help   help for exec
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

