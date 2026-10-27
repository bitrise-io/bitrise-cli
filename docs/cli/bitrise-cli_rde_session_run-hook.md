## bitrise-cli rde session run-hook

Queue a lifecycle hook to run again on a session (keeps the machine)

### Synopsis

Queue one lifecycle hook to run again on a session that runs lifecycle hooks
('session view' shows a "Setup:" line). The session's hook runner picks it up
on its next poll and runs the session's creation-time copy of the script with
the session's inputs; 'session view' lists the new attempt (triggered by
"run_again") and 'session logs --stage <hook> --attempt N' shows its output.

Use it after a setup failure ("Setup: setup failed" / "personalization
failed" / "restore script failed") whose cause was transient — a flaky
download, a service that was not up yet — because the machine and everything
on it stay as they are. A fix to the template's script does not reach an
existing session (it keeps its creation-time hooks): 'session rebuild' or a
new session is the way to pick it up.

  --hook   on_claim or on_restore (only hooks the session was created
           with), or personalize — the platform's own System setup step
           (credentials, Claude Code) — on a session that already ran it.

On create is not accepted: it runs once per machine, so a fresh attempt
needs a fresh machine — 'session rebuild' discards the machine and runs the
lifecycle again from On create ("On create cannot be run again on this
machine. Use Rebuild to run it on a fresh one.").

The backend refuses the request while a run is already pending or running on
the session (wait for the setup to settle), on a warm pool's unclaimed
sessions, on a session without lifecycle hooks and on a session without a
machine (restore it first).

```
bitrise-cli rde session run-hook SESSION_ID --hook on_claim|on_restore|personalize [flags]
```

### Examples

```
  bitrise-cli rde session run-hook SESSION_ID --hook on_restore
  bitrise-cli rde session run-hook SESSION_ID --hook personalize --output json
```

### Options

```
  -h, --help          help for run-hook
      --hook string   which hook to run again: on_claim, on_restore or personalize (required); On create cannot be run again — use 'session rebuild'
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

