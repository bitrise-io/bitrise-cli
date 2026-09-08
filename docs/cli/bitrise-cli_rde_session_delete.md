## bitrise-cli rde session delete

Permanently delete a session in any state (running sessions are stopped and discarded)

### Synopsis

Permanently delete a session in any state.

This is the way to get rid of a session you are done with. It works on running,
starting, terminating, terminated and failed sessions alike — no need to
'terminate' first. The session disappears immediately and cannot be restored.

If the VM is still running, the backend stops it and then discards it together
with its disk in the background; the command does not wait for that. Anything
still on the VM (uncommitted work, unsynced files) is lost, so push or download
what you need before deleting.

Only use 'session terminate' instead when you intend to 'session restore' the
same session later — a terminated session keeps its disk (and keeps using disk
space) until it is deleted.

The command refuses while the machine state is "unknown" (e.g. its node lost
connectivity); retry once the state settles.

```
bitrise-cli rde session delete SESSION_ID [flags]
```

### Options

```
  -h, --help   help for delete
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

