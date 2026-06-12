## bitrise-cli

Bitrise platform CLI

### Synopsis

bitrise-cli is the Bitrise platform CLI — manage builds, apps, and pipelines from your terminal.

Get started: run "bitrise-cli auth login" to store your access token.

For scripts and agents:
  Pass --output json for stable, machine-readable output (or run
  "bitrise-cli config set output json" once). Data goes to stdout and
  diagnostics to stderr — even in json mode — so output stays pipeable.
  Most build and yml commands act on one app: pass --app ID or set BITRISE_APP_ID.

Configuration precedence: flag > env > per-dir (.bitrise-cli.yml) > global
config.yaml > built-in default. Run "bitrise-cli config" for all keys and env vars.

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
* [bitrise-cli rde](bitrise-cli_rde.md)	 - Manage Bitrise Remote Dev Environments (sessions, templates, …)
* [bitrise-cli stack](bitrise-cli_stack.md)	 - List available stacks
* [bitrise-cli step](bitrise-cli_step.md)	 - Search steps and inspect their inputs
* [bitrise-cli user](bitrise-cli_user.md)	 - Create and manage your Bitrise account
* [bitrise-cli version](bitrise-cli_version.md)	 - Print version, commit, and build info
* [bitrise-cli yml](bitrise-cli_yml.md)	 - Get, update, or validate the bitrise.yml stored on Bitrise

