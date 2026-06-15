## bitrise-cli build

Trigger, list, and inspect builds

### Synopsis

Trigger, list, and inspect builds.

Builds belong to an app: pass --app ID on any build command, or set
BITRISE_APP_ID (or run "bitrise-cli config set app_id ID"). Running
"bitrise-cli build" with no subcommand lists builds for that app.

```
bitrise-cli build [flags]
```

### Examples

```
  bitrise-cli build list --app APP_ID
  bitrise-cli build trigger --app APP_ID --branch main --workflow primary
  bitrise-cli build view --app APP_ID BUILD_ID --output json
  bitrise-cli build log --app APP_ID BUILD_ID
```

### Options

```
      --app string   app ID (or set BITRISE_APP_ID)
  -h, --help         help for build
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
* [bitrise-cli build abort](bitrise-cli_build_abort.md)	 - Abort a running or queued build
* [bitrise-cli build list](bitrise-cli_build_list.md)	 - List builds for an app
* [bitrise-cli build log](bitrise-cli_build_log.md)	 - Print the build log
* [bitrise-cli build trigger](bitrise-cli_build_trigger.md)	 - Start a new build
* [bitrise-cli build view](bitrise-cli_build_view.md)	 - Show details of a single build
* [bitrise-cli build watch](bitrise-cli_build_watch.md)	 - Stream logs for a running build
* [bitrise-cli build yml](bitrise-cli_build_yml.md)	 - Print the bitrise.yml a specific build ran with

