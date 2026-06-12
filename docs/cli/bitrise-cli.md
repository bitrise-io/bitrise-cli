## bitrise-cli

Bitrise platform CLI

### Synopsis

bitrise-cli is the Bitrise platform CLI — manage builds, apps, and pipelines from your terminal.

Tip:
  Install a "br" alias for less typing where the name is free, e.g.
    ln -s "$(command -v bitrise-cli)" /usr/local/bin/br
  or as a shell alias:
    alias br=bitrise-cli

Output formats:
  --output human  human-readable, default (tables and key/value lines)
  --output json   machine-readable; the schema is part of the CLI's stable contract

Configuration (precedence: flag > env > per-dir > global > built-in default):
  Global file:   $XDG_CONFIG_HOME/bitrise/config.yaml (or ~/.config/bitrise/config.yaml)
  Per-dir file:  .bitrise-cli.yml in the current directory or any ancestor
  Manage with:   bitrise-cli config set <key> <value>   (run "bitrise-cli config" for details)
  Env overrides: BITRISE_TOKEN, BITRISE_APP_ID, BITRISE_WORKSPACE_ID,
                 BITRISE_OUTPUT, BITRISE_API_BASE_URL, BITRISE_RDE_API_BASE_URL,
                 BITRISE_WEB_BASE_URL, BITRISE_CLI_THEME

Color theme:
  --theme auto    detect terminal background via OSC 11 (default)
  --theme dark    force the dark-mode palette
  --theme light   force the light-mode palette (use on white-bg terminals)
  --theme none    disable ANSI colors entirely (same as --no-color)

Tip for automation / agents:
  Pass --output json on every command — or run "bitrise-cli config set output json"
  once — to get parseable output. Data is written to stdout; errors and diagnostics
  always go to stderr, even in JSON mode.

### Options

```
  -h, --help            help for bitrise-cli
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli api](bitrise-cli_api.md)	 - Make an authenticated request to the Bitrise API
* [bitrise-cli app](bitrise-cli_app.md)	 - List, inspect, and manage apps
* [bitrise-cli auth](bitrise-cli_auth.md)	 - Manage the Bitrise access token
* [bitrise-cli build](bitrise-cli_build.md)	 - Trigger, list, and inspect builds
* [bitrise-cli completion](bitrise-cli_completion.md)	 - Generate the autocompletion script for the specified shell
* [bitrise-cli config](bitrise-cli_config.md)	 - Manage CLI configuration (defaults persisted to a YAML file)
* [bitrise-cli purr](bitrise-cli_purr.md)	 - Visit Purr Request, the Bitrise CLI mascot
* [bitrise-cli rde](bitrise-cli_rde.md)	 - Manage Bitrise Remote Dev Environments (sessions, templates, …)
* [bitrise-cli stack](bitrise-cli_stack.md)	 - List available stacks
* [bitrise-cli step](bitrise-cli_step.md)	 - Search steps and inspect their inputs
* [bitrise-cli user](bitrise-cli_user.md)	 - Create and manage your Bitrise account
* [bitrise-cli version](bitrise-cli_version.md)	 - Print version, commit, and build info
* [bitrise-cli yml](bitrise-cli_yml.md)	 - Get, update, or validate the bitrise.yml stored on Bitrise

