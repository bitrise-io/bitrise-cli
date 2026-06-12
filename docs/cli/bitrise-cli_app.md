## bitrise-cli app

List, inspect, and manage apps

### Synopsis

List, inspect, and manage the apps you can access.

Running "bitrise-cli app" with no subcommand lists your apps.

```
bitrise-cli app [flags]
```

### Examples

```
  bitrise-cli app
  bitrise-cli app list --output json
  bitrise-cli app view APP_ID
  bitrise-cli app create --repo-url https://github.com/acme/widget.git --workspace WORKSPACE_ID
```

### Options

```
  -h, --help   help for app
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
* [bitrise-cli app create](bitrise-cli_app_create.md)	 - Register a new app on Bitrise
* [bitrise-cli app list](bitrise-cli_app_list.md)	 - List apps the authenticated user can access
* [bitrise-cli app view](bitrise-cli_app_view.md)	 - Show details of a single app

