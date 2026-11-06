## bitrise-cli rde workspace-script view

Show the workspace script, its state and its variables

### Synopsis

Show the workspace script: whether it is enabled, who changed it last and
when, its variables and the script itself.

Secret variable values are hidden unless --show-secrets is given; protected
variables are write-only and never shown — only whether a value is set.

```
bitrise-cli rde workspace-script view [flags]
```

### Examples

```
  bitrise-cli rde workspace-script view
  bitrise-cli rde workspace-script view --show-secrets
  # Compare the live script with the one in git.
  bitrise-cli rde workspace-script view -o json | jq -r .script | diff - workspace-script.sh
```

### Options

```
  -h, --help           help for view
      --show-secrets   show secret variable values (protected values are never returned)
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

* [bitrise-cli rde workspace-script](bitrise-cli_rde_workspace-script.md)	 - View and edit the script every session in the workspace boots with

