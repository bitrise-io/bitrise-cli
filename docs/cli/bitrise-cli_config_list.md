## bitrise-cli config list

List the current config-file values

### Synopsis

List the values currently saved in the config file.

Env-var overrides are NOT shown by this command — they only apply at runtime
to other bitrise-cli commands.

```
bitrise-cli config list [flags]
```

### Options

```
  -h, --help   help for list
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

