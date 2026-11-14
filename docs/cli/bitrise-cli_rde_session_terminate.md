## bitrise-cli rde session terminate

Terminate a running session but keep it for a later restore

### Synopsis

Terminate a running session but keep it for a later restore.

The VM is stopped and its disk is preserved: the session stays in the list as
"terminated" and can be brought back with 'session restore'. Use this when you
intend to come back to this exact session (e.g. to keep uncommitted work or an
expensive warm state). A terminated session keeps using disk space until it is
deleted.

If you are simply done with the session, use 'session delete' instead — it
works on running sessions directly and frees the disk; no terminate needed.

Terminate is asynchronous: by default the command returns while the session
is still "terminating". Pass --wait to block until the session settles into a
terminal state ("terminated" or "failed"), e.g. before restoring it or when
you want the disk snapshot to be complete before moving on.

Terminating someone else's session: on a workspace where it is enabled (it is
enabled per workspace on request), whoever can run 'rde usage' can terminate
any session in the workspace: other members' personal sessions,
workspace-owned sessions and preview link sessions. That is a personal access
token of someone holding the workspace's billing-view permission, or a
Workspace API Token created with Remote Dev Environments access and a
workspace role that can manage the workspace (Manager, or a custom role with
"Modify workspace settings"); any other Workspace API Token does not get this
wider right. 'rde usage --sessions' lists the sessions with their IDs. Pass
the session ID: a session that is not yours cannot be found by name.

Only terminate is widened. The session is stopped, never deleted, and its
owner can restore it; viewing, deleting, restoring, exec, logs and the rest
still answer "not found" for a session that is not yours. For the same reason
--wait cannot follow such a session: the command warns and returns while it
is still "terminating". For someone else's session only its ID, name, status,
owner and timestamps come back, so that is all this command prints for it.

Where it is not enabled, the API refuses with 403 "Terminating another
member's session is not enabled for this workspace."; without the permission,
or for an unknown ID, it answers 404 as before.

A warm session a pool still holds (not claimed yet) cannot be terminated this
way; lower the pool's size instead ('rde warm-pool set-size WARM_POOL_ID 0').

```
bitrise-cli rde session terminate SESSION_ID [flags]
```

### Examples

```
  bitrise-cli rde session terminate SESSION_ID
  bitrise-cli rde session terminate SESSION_ID --wait
  # Someone else's session: its ID is in the usage report
  bitrise-cli rde usage --sessions
```

### Options

```
  -h, --help                    help for terminate
      --wait                    block until the session settles into a terminal state (terminated/failed) before returning
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

