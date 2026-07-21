## bitrise-cli rde session list

List RDE sessions in the workspace

### Synopsis

List every RDE session the authenticated user has in the workspace.

Filter by labels with --label-selector key=value (repeatable; selectors are
exact matches and are ANDed, at most 8 per request).

The session list comes from the backend in arbitrary order; the CLI does
not paginate (the API doesn't paginate this endpoint either).

```
bitrise-cli rde session list [flags]
```

### Examples

```
  bitrise-cli rde session list
  bitrise-cli rde session list --workspace my-workspace
  bitrise-cli rde session list -l team=mobile -l branch=main
  bitrise-cli rde session list --output json | jq '.items[].id'
```

### Options

```
  -h, --help                         help for list
  -l, --label-selector stringArray   only sessions whose labels match key=value exactly (repeatable; multiple selectors must all match)
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

