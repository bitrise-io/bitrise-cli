## bitrise-cli build abort

Abort a running or queued build

### Synopsis

Abort a running or queued build.

Required arguments:
  BUILD_SLUG         build slug to abort

Required flags:
  --app SLUG         (or BITRISE_APP_SLUG env var)

Optional flags:
  --reason STRING            reason for aborting (recorded in the build log)
  --abort-with-success       mark the aborted build as successful
  --skip-git-status-report   skip sending a git status report
  --skip-notifications       skip sending build notifications

```
bitrise-cli build abort BUILD_SLUG [flags]
```

### Examples

```
  bitrise-cli build abort BUILD_SLUG --app my-app-slug
  bitrise-cli build abort BUILD_SLUG --app my-app-slug --reason "no longer needed"
  bitrise-cli build abort BUILD_SLUG --app my-app-slug --abort-with-success
  bitrise-cli build abort BUILD_SLUG --app my-app-slug --output json
```

### Options

```
      --abort-with-success       mark the aborted build as successful
  -h, --help                     help for abort
      --reason string            reason for aborting the build
      --skip-git-status-report   skip sending a git status report
      --skip-notifications       skip sending build notifications
```

### Options inherited from parent commands

```
      --app string      app slug (also accepted as --project; or set BITRISE_APP_SLUG)
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli build](bitrise-cli_build.md)	 - Trigger, list, and inspect builds

