## bitrise-cli rde stack list

List machine stacks

```
bitrise-cli rde stack list [flags]
```

### Examples

```
  bitrise-cli rde stack list
  bitrise-cli rde stack list --output json | jq '.items[].id'
```

### Options

```
  -h, --help   help for list
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

* [bitrise-cli rde stack](bitrise-cli_rde_stack.md)	 - List machine stacks available to the workspace

