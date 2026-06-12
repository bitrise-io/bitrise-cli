## bitrise-cli stack

List available stacks

### Synopsis

List the build stacks available to you.

Running "bitrise-cli stack" with no subcommand lists stacks.

```
bitrise-cli stack [flags]
```

### Examples

```
  bitrise-cli stack list
  bitrise-cli stack list --output json
  bitrise-cli stack list --workspace WORKSPACE_ID
```

### Options

```
  -h, --help   help for stack
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli](bitrise-cli.md)	 - Bitrise platform CLI
* [bitrise-cli stack list](bitrise-cli_stack_list.md)	 - List available stacks and their machine configurations

