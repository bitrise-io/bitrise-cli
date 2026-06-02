## bitrise-cli auth logout

Remove the saved access token

### Synopsis

Remove the auth.yaml file. Does not affect tokens set via the
BITRISE_TOKEN environment variable or the legacy 'config set token'.

```
bitrise-cli auth logout [flags]
```

### Options

```
  -h, --help   help for logout
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli auth](bitrise-cli_auth.md)	 - Manage the Bitrise access token

