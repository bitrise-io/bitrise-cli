## bitrise-cli rde session diff

Compare a session's template snapshot with the current template

### Synopsis

Show what changed between the template config snapshotted at the session's
creation time and the template's current config. Most useful when a session
reports template_outdated=true.

Lists which template variable keys changed (values are never exposed) and
the simple per-field differences (image, machine type, scripts, working
directory). When the template was deleted, only the snapshot is shown.

```
bitrise-cli rde session diff SESSION_ID [flags]
```

### Options

```
  -h, --help   help for diff
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

