## bitrise-cli rde machine-type list

List machine types compatible with a given stack

### Synopsis

List machine types compatible with the stack given by --stack.

Each machine type is offered by one or more clusters. The cluster name is
shown only when a machine type is offered by more than one cluster for the
selected stack — pass that name as --cluster to 'rde session create' to
pin a target.

```
bitrise-cli rde machine-type list --stack STACK_ID [flags]
```

### Examples

```
  bitrise-cli rde machine-type list --stack osx-xcode-16.0.x-edge
```

### Options

```
  -h, --help           help for list
      --stack string   stack ID to list compatible machine types for (required)
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

* [bitrise-cli rde machine-type](bitrise-cli_rde_machine-type.md)	 - List machine types compatible with a given stack

