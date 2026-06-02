## bitrise-cli rde session delete-terminated

Permanently delete every terminated session in the workspace

### Synopsis

Permanently delete every terminated session in the workspace.
This cannot be undone. Pass --yes to skip the confirmation prompt.

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
      --workspace string   workspace slug (or set BITRISE_WORKSPACE_ID; defaults to default_workspace_slug)
```

### SEE ALSO

* [bitrise-cli rde session](bitrise-cli_rde_session.md)	 - Create, list, inspect, and manage RDE sessions

