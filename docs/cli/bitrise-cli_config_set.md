## bitrise-cli config set

Set a config key and save the file

### Synopsis

Set a config key and save the file.

Valid keys: output, app_slug, default_workspace_slug, api_base_url, rde_api_base_url, web_base_url, theme

The value is validated before being saved (e.g. "output" must be human or json,
"api_base_url" and "web_base_url" must be valid URLs). The file is written with
0600 permissions.

If VALUE is "-", the value is read from stdin (trailing newline trimmed).

```
bitrise-cli config set KEY VALUE [flags]
```

### Examples

```
  bitrise-cli config set output json
  bitrise-cli config set app_slug 5db8b1d8-cae8-4cea-b943-ddc8f48e5e7c
```

### Options

```
  -h, --help   help for set
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

