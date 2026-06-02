## bitrise-cli user me

Show the currently authenticated user

### Synopsis

Show the profile of the user whose token is in use.

The token is resolved from BITRISE_TOKEN, auth.yaml, or config.yaml — run
'bitrise-cli auth status' to confirm which source is active.

```
bitrise-cli user me [flags]
```

### Examples

```
  bitrise-cli user me
  bitrise-cli user me --output json
```

### Options

```
  -h, --help   help for me
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli user](bitrise-cli_user.md)	 - Create and manage your Bitrise account

