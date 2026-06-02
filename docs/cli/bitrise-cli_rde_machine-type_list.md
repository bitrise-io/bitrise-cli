## bitrise-cli rde machine-type list

List machine types compatible with a given image

### Synopsis

List machine types compatible with the image given by --image.

Each machine type is offered by one or more clusters. The cluster name is
shown only when a machine type is offered by more than one cluster for the
selected image — pass that name as --cluster to 'rde session create' to
pin a target.

```
bitrise-cli rde machine-type list --image NAME [flags]
```

### Examples

```
  bitrise-cli rde machine-type list --image osx-xcode-edge
```

### Options

```
  -h, --help           help for list
      --image string   image name to list compatible machine types for (required)
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

* [bitrise-cli rde machine-type](bitrise-cli_rde_machine-type.md)	 - List machine types compatible with a given image

