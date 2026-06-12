## bitrise-cli auth status

Show whether an access token is configured and where it came from

### Synopsis

Show whether an access token is configured and which source supplied it.

Sources, in precedence order:
  env        BITRISE_TOKEN environment variable
  auth file  auth.yaml, written by 'bitrise-cli auth login' (OAuth or a
             pasted/email token — a new login overwrites the previous one).
             OAuth logins are shown as "oauth (auth file)" and refreshed
             automatically.
  none       no token configured

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

