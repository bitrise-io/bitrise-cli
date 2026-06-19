## bitrise-cli rde saved-input create

Create a new saved input

### Synopsis

Create a new saved input.

The value can be supplied three ways:
  --value VALUE   use VALUE literally (pass --value - to store a literal dash)
  --value-stdin   read the value from stdin without prompting; keeps secrets
                  out of shell history
  neither         prompt for the value interactively; input is masked when
                  stdin is a terminal

```
bitrise-cli rde saved-input create [flags]
```

### Examples

```
  bitrise-cli rde saved-input create --key repo-name --value my-app
  echo -n "ghp_xxx" | bitrise-cli rde saved-input create --key gh-token --value-stdin --secret
  bitrise-cli rde saved-input create --key gh-token --secret   # prompts for the value
```

### Options

```
  -h, --help           help for create
      --key string     saved-input key (required)
      --secret         encrypt value at rest; the value will be masked in subsequent reads
      --value string   value to store (literal)
      --value-stdin    read the value from stdin without prompting
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

* [bitrise-cli rde saved-input](bitrise-cli_rde_saved-input.md)	 - Manage saved inputs (reusable credentials/values)

