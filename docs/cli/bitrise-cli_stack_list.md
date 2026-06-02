## bitrise-cli stack list

List available stacks and their machine configurations

### Synopsis

List all available stacks with their OS, status, and version information.

When --workspace is provided, returns stacks available for that workspace,
including any custom stacks configured for it.
Without --workspace, returns globally available stacks.

```
bitrise-cli stack list [flags]
```

### Examples

```
  bitrise-cli stack list
  bitrise-cli stack list --workspace my-workspace-slug
  bitrise-cli stack list --output json
```

### Options

```
  -h, --help               help for list
      --workspace string   workspace slug for workspace-specific stacks (including custom stacks)
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli stack](bitrise-cli_stack.md)	 - List available stacks

