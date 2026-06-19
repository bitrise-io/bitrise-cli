## bitrise-cli rde session

Create, list, inspect, and manage RDE sessions

### Synopsis

Create, list, inspect, and manage RDE sessions.

Commands that take a SESSION_ID also accept a session name — it's resolved to
an ID for you. Names aren't unique, so if more than one session shares the name
the command errors and lists the candidate IDs to pick from.

```
bitrise-cli rde session [flags]
```

### Options

```
  -h, --help   help for session
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
* [bitrise-cli rde session create](bitrise-cli_rde_session_create.md)	 - Create a new RDE session from a template
* [bitrise-cli rde session delete](bitrise-cli_rde_session_delete.md)	 - Permanently delete a session
* [bitrise-cli rde session delete-terminated](bitrise-cli_rde_session_delete-terminated.md)	 - Permanently delete every terminated session in the workspace
* [bitrise-cli rde session diff](bitrise-cli_rde_session_diff.md)	 - Compare a session's template snapshot with the current template
* [bitrise-cli rde session download](bitrise-cli_rde_session_download.md)	 - Download a file or directory from a session
* [bitrise-cli rde session exec](bitrise-cli_rde_session_exec.md)	 - Run a bash command on a session over SSH
* [bitrise-cli rde session list](bitrise-cli_rde_session_list.md)	 - List RDE sessions in the workspace
* [bitrise-cli rde session logs](bitrise-cli_rde_session_logs.md)	 - Print a session's warmup or startup logs
* [bitrise-cli rde session notifications](bitrise-cli_rde_session_notifications.md)	 - List notifications emitted by a session
* [bitrise-cli rde session open-vnc](bitrise-cli_rde_session_open-vnc.md)	 - Open a session's VNC endpoint in the OS-default viewer
* [bitrise-cli rde session restore](bitrise-cli_rde_session_restore.md)	 - Restore a terminated session (re-provisions its VM from the persistent disk)
* [bitrise-cli rde session terminate](bitrise-cli_rde_session_terminate.md)	 - Terminate a running session (preserves it for later restart)
* [bitrise-cli rde session update](bitrise-cli_rde_session_update.md)	 - Update a session's name, description, or auto-terminate duration
* [bitrise-cli rde session upload](bitrise-cli_rde_session_upload.md)	 - Upload a local file or directory into a session
* [bitrise-cli rde session view](bitrise-cli_rde_session_view.md)	 - Show details of a single session
* [bitrise-cli rde session vnc](bitrise-cli_rde_session_vnc.md)	 - Print VNC connection credentials for a session

