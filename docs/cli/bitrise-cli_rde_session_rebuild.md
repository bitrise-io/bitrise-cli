## bitrise-cli rde session rebuild

Replace a session's machine and run its lifecycle hooks again from On create

### Synopsis

Replace the machine of a session that runs lifecycle hooks and run its whole
setup again: the current VM AND its disk are discarded (anything unsaved on
it is lost), a fresh machine boots for the same session — same id, name,
labels and creation-time configuration (scripts, inputs, feature flags,
device) — and On create runs again from scratch, followed by the platform's
System setup and On claim ('session view' tags those runs "rebuild").

Use it when On create failed — it is the only way to run On create again
('session run-hook' does not accept it: On create runs once per machine) —
or when you want a clean environment with the same configuration. When a
failed On claim, On restore or System setup needs another go, 'session
run-hook' keeps the machine and everything on it.

Rebuild is asynchronous: by default the command returns while the session is
"starting". Pass --wait to block until it finishes provisioning (the command
exits non-zero if it ends in any state other than "running"); the hooks then
still run — watch "Setup:" in 'session view' until it says ready.

Refused on a warm pool's unclaimed sessions, on device preview sessions and
on sessions without lifecycle hooks.

```
bitrise-cli rde session rebuild SESSION_ID [flags]
```

### Examples

```
  bitrise-cli rde session rebuild SESSION_ID
  bitrise-cli rde session rebuild SESSION_ID --wait
```

### Options

```
  -h, --help                    help for rebuild
      --wait                    wait until the new machine leaves provisioning (running, failed, …) and its hooks are done before returning; exits 1 if the final status isn't running or a hook failed
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

