## bitrise-cli rde saved-input update

Update a saved input's value and/or secret flag

### Synopsis

Update a saved input's value and/or secret flag.

Pass --value VALUE to set a new value, or --value-stdin to read it from stdin
without prompting (keeps secrets out of shell history). Omit both to change only
the --secret flag.

```
bitrise-cli rde saved-input update SAVED_INPUT_ID [flags]
```

### Examples

```
  bitrise-cli rde saved-input update ID --value new-value
  echo -n "ghp_xxx" | bitrise-cli rde saved-input update ID --value-stdin --secret
```

### Options

```
  -h, --help           help for update
      --secret         set/unset the secret flag
      --value string   new value (literal)
      --value-stdin    read the new value from stdin without prompting
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

* [bitrise-cli rde saved-input](bitrise-cli_rde_saved-input.md)	 - Manage saved inputs (reusable credentials/values)

