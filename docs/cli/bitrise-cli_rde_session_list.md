## bitrise-cli rde session list

List RDE sessions in the workspace

### Synopsis

List RDE sessions in the workspace.

Filter by labels with --label-selector key=value (repeatable; selectors are
exact matches and are ANDed, at most 8 per request).

By default every session the authenticated user has in the workspace is
listed. Pass --scope workspace for sessions owned by the workspace itself
rather than by a user (for example sessions spawned by workspace device
preview links) — every workspace member sees the same list. The owning user
or workspace is reported via the owner_type and owner_id fields in
--output json.

The session list comes from the backend in arbitrary order; the CLI does
not paginate (the API doesn't paginate this endpoint either).

```
bitrise-cli rde session list [flags]
```

### Examples

```
  bitrise-cli rde session list
  bitrise-cli rde session list --workspace my-workspace
  bitrise-cli rde session list --scope workspace
  bitrise-cli rde session list -l team=mobile -l branch=main
  bitrise-cli rde session list --output json | jq '.items[].id'
```

### Options

```
  -h, --help                         help for list
  -l, --label-selector stringArray   only sessions whose labels match key=value exactly (repeatable; multiple selectors must all match)
      --scope string                 which sessions to list: mine (sessions you created) or workspace (sessions owned by the workspace itself, visible to every member) (default "mine")
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

