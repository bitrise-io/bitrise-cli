## bitrise-cli rde session delete-terminated

Permanently delete your terminated sessions in this workspace

### Synopsis

Permanently delete your terminated sessions in this workspace.

Only sessions YOU created are affected: the server scopes the call to the
caller's own sessions, so other members' sessions (and workspace-owned
device-preview sessions) are never touched. This cannot be undone. Pass
--yes to skip the confirmation prompt.

```
bitrise-cli rde session delete-terminated [flags]
```

### Options

```
  -h, --help   help for delete-terminated
      --yes    skip the confirmation prompt
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

