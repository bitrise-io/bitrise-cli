## bitrise-cli rde claude

Create an ephemeral RDE session and attach to Claude Code

### Synopsis

Create a fresh RDE session from the "szabi linux empty" template, wait for it
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

```
bitrise-cli rde claude [flags]
```

### Examples

```
  bitrise-cli rde claude --workspace WORKSPACE_ID
```

### Options

```
  -h, --help                    help for claude
      --wait-timeout duration   max time to wait for the session to start (uses Go duration syntax: 30s, 5m, 1h) (default 10m0s)
```

### Options inherited from parent commands

```
      --no-color           disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string      output format: human|json (default "human")
  -q, --quiet              suppress non-error diagnostic messages
      --theme string       color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
      --workspace string   workspace ID (or set BITRISE_WORKSPACE_ID; defaults to default_workspace_id)
```

### SEE ALSO

* [bitrise-cli rde](bitrise-cli_rde.md)	 - Manage Bitrise Remote Dev Environments (sessions, templates, …)

