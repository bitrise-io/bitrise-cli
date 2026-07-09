## bitrise-cli rde session restore

Restore a terminated session (re-provisions its VM from the persistent disk)

### Synopsis

Restore a terminated session (re-provisions its VM from the persistent disk).

Restore is asynchronous: by default the command returns while the session is
still "starting". Pass --wait to block until the session finishes provisioning
(mirrors 'session create --wait'); the command exits non-zero if the session
ends in any state other than "running". This lets an unattended caller restore
and then immediately use the session without hand-rolling a poll loop.

```
bitrise-cli rde session restore SESSION_ID [flags]
```

### Options

```
  -h, --help                    help for restore
      --wait                    wait until the session leaves provisioning (running, failed, …) before returning; exits 1 if the final status isn't running
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

