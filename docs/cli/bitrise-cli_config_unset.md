## bitrise-cli config unset

Remove a config key and save the file

### Synopsis

Remove a config key and save the file.

Valid keys: output, app_id, default_workspace_id, api_base_url, rde_api_base_url, web_base_url, theme

```
bitrise-cli config unset KEY [flags]
```

### Options

```
  -h, --help   help for unset
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli config](bitrise-cli_config.md)	 - Manage CLI configuration (defaults persisted to a YAML file)

