## bitrise-cli rde session notifications

List notifications emitted by a session

### Synopsis

List notifications a session has emitted (agent stop, permission prompt,
idle, …). Use --since with the timestamp of the newest notification you've
seen to poll for new events incrementally.

```
bitrise-cli rde session notifications SESSION_ID [flags]
```

### Examples

```
  bitrise-cli rde session notifications SESSION_ID
  bitrise-cli rde session notifications SESSION_ID --since 2026-05-27T10:00:00Z --limit 100
  bitrise-cli rde session notifications SESSION_ID --order asc
```

### Options

```
      --before string   only notifications created before this RFC3339 timestamp (exclusive)
  -h, --help            help for notifications
      --limit int       max notifications to return (server default 50, max 100)
      --order string    sort order: asc (oldest first) or desc (newest first); server default is desc
      --since string    only notifications created after this RFC3339 timestamp (exclusive)
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

