## bitrise-cli rde session delete-terminated

Permanently delete terminated sessions in this workspace

### Synopsis

Permanently delete terminated sessions in this workspace.

By default only sessions YOU created are affected: the server scopes the
call to the caller's own sessions, so other members' sessions and
workspace-owned sessions are never touched. Pass --scope workspace to
delete the terminated sessions owned by the workspace itself instead (the
ones 'rde session list --scope workspace' shows) — any member may do that.
With a Workspace API Token the workspace scope is the default and the only
one available. This cannot be undone. Pass --yes to skip the confirmation
prompt.

```
bitrise-cli rde session delete-terminated [flags]
```

### Examples

```
  bitrise-cli rde session delete-terminated
  bitrise-cli rde session delete-terminated --scope workspace --yes
```

### Options

```
  -h, --help           help for delete-terminated
      --scope string   which terminated sessions to delete: mine (sessions you created) or workspace (sessions owned by the workspace itself) (default "mine")
      --yes            skip the confirmation prompt
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

