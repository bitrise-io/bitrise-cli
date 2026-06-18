## bitrise-cli rde claude

Create an RDE session and attach to Claude Code

### Synopsis

Create a fresh RDE session, wait for it to start, then SSH in and drop you
directly into Claude Code (not a shell).

You pick the image first, then a machine type compatible with it. Your choice
is remembered per repository and preselected next time, so you can just press
Enter. Pass --image / --machine-type to skip the prompts
(useful for scripts); when stdin isn't a terminal the remembered or default
selection is used without prompting.

Run this from inside a git repository: the session clones the same repository
and branch you're on (via 'git clone') and starts Claude Code inside that
clone. Only the pushed remote state of the branch is cloned — local
uncommitted or unpushed changes are not transferred.

When you exit Claude Code, the session is terminated automatically (its VM is
torn down), but the session is preserved and can be restored later. Each
invocation creates a new, uniquely named session (claude-<id>).

Resume a previous session instead of creating one:

  --continue        resume the most recent session started from this repo
  --resume          pick a previous session for this repo from a list
  --resume SESSION  resume a specific session by ID (or name)

Resuming reconnects to the session if it's still running, otherwise restores it
and continues the same Claude Code conversation. Sessions are tracked locally
per repository as you use them; while a session is live, its AI-generated title
and a "repo @ branch" description (with the pull-request URL) are kept up to
date both locally and on the session itself.

A local SSH agent ($SSH_AUTH_SOCK), if present, is forwarded into the session
so the clone (and git-over-SSH inside the session) uses your local keys. If the
repo's origin is an HTTPS GitHub/GitLab/Bitbucket URL, it's rewritten to its
SSH form so the forwarded agent can authenticate.

Unless a Claude Code token is already configured on the control plane, a local
credential is saved there before the session is created — taken from
$CLAUDE_CODE_OAUTH_TOKEN or $ANTHROPIC_API_KEY, then ~/.claude/.credentials.json,
or minted with 'claude setup-token' (browser auth). The control plane uses that
token to install Claude Code and tmux during provisioning and to authenticate
the in-session claude; once saved, future sessions reuse it.

```
bitrise-cli rde claude [SESSION_ID] [flags]
```

### Examples

```
  bitrise-cli rde claude --workspace WORKSPACE_ID
  bitrise-cli rde claude --image osx-26-edge --machine-type g2.mac.m2pro.4c-6g
  bitrise-cli rde claude --continue
  bitrise-cli rde claude --resume
```

### Options

```
      --continue                resume the most recent session started from this repo
  -h, --help                    help for claude
      --image string            image to use (skips the image prompt); see 'rde image list'
      --machine-type string     machine type to use (skips the machine-type prompt); see 'rde machine-type list'
      --resume                  resume a previous session for this repo; with no SESSION_ID, pick one from a list
      --wait-timeout duration   max time to wait for the session to start (uses Go duration syntax: 30s, 5m, 1h) (default 10m0s)
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

* [bitrise-cli rde](bitrise-cli_rde.md)	 - Manage Bitrise Remote Dev Environments (sessions, templates, …)

