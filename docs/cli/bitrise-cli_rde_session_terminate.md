## bitrise-cli rde session terminate

Terminate a running session (preserves it for later restart)

### Synopsis

Terminate a running session (preserves it for later restart).

Terminate is asynchronous: by default the command returns while the session
is still "terminating". Pass --wait to block until the session settles into a
terminal state ("terminated" or "failed"). This is what makes a
'terminate --wait && delete' pipeline reliable — delete rejects any session
that isn't yet terminated or failed.

```
bitrise-cli rde session terminate SESSION_ID [flags]
```

### Options

```
  -h, --help                    help for terminate
      --wait                    block until the session settles into a terminal state (terminated/failed) before returning; makes 'terminate --wait && delete' reliable
      --wait-timeout duration   max time to wait when --wait is set (Go duration syntax: 30s, 5m, 1h) (default 10m0s)
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

