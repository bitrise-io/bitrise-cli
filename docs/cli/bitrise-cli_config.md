## bitrise-cli config

Manage CLI configuration (defaults persisted to a YAML file)

### Synopsis

Manage persistent CLI configuration.

Storage:
  Global file: YAML at $XDG_CONFIG_HOME/bitrise/config.yaml
               (falls back to ~/.config/bitrise/config.yaml).
  Per-dir:     .bitrise-cli.yml in the current directory or any ancestor.
               Useful for per-project app_slug pinning.

Precedence at runtime:
  flag > env > per-directory file > global file > built-in default

Recognized keys:
  output, app_slug, default_workspace_slug, api_base_url, rde_api_base_url, web_base_url, theme

Environment overrides for the same values:
  BITRISE_OUTPUT, BITRISE_APP_SLUG, BITRISE_TOKEN, BITRISE_API_BASE_URL, BITRISE_WEB_BASE_URL, BITRISE_CLI_THEME

Note: 'set'/'unset' modify only the global file. Per-directory files must be
edited by hand.

To manage your access token, use 'bitrise-cli auth login/logout/status'.

### Options

```
  -h, --help   help for config
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
* [bitrise-cli config get](bitrise-cli_config_get.md)	 - Print the value of a single config key (raw, unmasked)
* [bitrise-cli config list](bitrise-cli_config_list.md)	 - List the current config-file values
* [bitrise-cli config path](bitrise-cli_config_path.md)	 - Print the absolute path of the config file
* [bitrise-cli config set](bitrise-cli_config_set.md)	 - Set a config key and save the file
* [bitrise-cli config unset](bitrise-cli_config_unset.md)	 - Remove a config key and save the file

