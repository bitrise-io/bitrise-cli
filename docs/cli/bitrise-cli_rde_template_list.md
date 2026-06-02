## bitrise-cli rde template list

List RDE templates in the workspace

```
bitrise-cli rde template list [flags]
```

### Examples

```
  bitrise-cli rde template list
  bitrise-cli rde template list --output json | jq '.items[].id'
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
      --workspace string   workspace slug (or set BITRISE_WORKSPACE_ID; defaults to default_workspace_slug)
```

### SEE ALSO

* [bitrise-cli rde template](bitrise-cli_rde_template.md)	 - List and inspect RDE templates

