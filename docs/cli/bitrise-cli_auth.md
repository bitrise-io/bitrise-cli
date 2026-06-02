## bitrise-cli auth

Manage the Bitrise access token

### Synopsis

Manage the Bitrise access token used for API requests.

Both Personal Access Tokens (PAT) and Workspace API Tokens (WAT) work the
same way on the wire — paste either kind here.

Storage:
  YAML file at $XDG_CONFIG_HOME/bitrise/auth.yaml (or ~/.config/bitrise/auth.yaml).
  Written with 0600 permissions, separate from preferences in config.yaml.

Env override:
  BITRISE_TOKEN takes precedence over the saved token; useful for CI.

### Options

```
  -h, --help   help for auth
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
* [bitrise-cli auth login](bitrise-cli_auth_login.md)	 - Save a Bitrise access token
* [bitrise-cli auth logout](bitrise-cli_auth_logout.md)	 - Remove the saved access token
* [bitrise-cli auth status](bitrise-cli_auth_status.md)	 - Show whether an access token is configured and where it came from

