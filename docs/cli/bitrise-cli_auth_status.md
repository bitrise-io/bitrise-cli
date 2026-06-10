## bitrise-cli auth status

Show whether an access token is configured and where it came from

### Synopsis

Show whether an access token is configured and which source supplied it.

Sources, in precedence order:
  env                BITRISE_TOKEN environment variable
  oauth (auth file)  auth.yaml, signed in via 'auth login --oauth' (auto-refreshed)
  auth file          auth.yaml (set via 'bitrise-cli auth login')
  none               no token configured

```
bitrise-cli auth status [flags]
```

### Options

```
  -h, --help   help for status
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

