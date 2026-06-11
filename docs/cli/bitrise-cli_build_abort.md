## bitrise-cli build abort

Abort a running or queued build

### Synopsis

Abort a running or queued build.

Required arguments:
  BUILD_ID           build ID to abort

Required flags:
  --app ID           (or BITRISE_APP_ID env var)

Optional flags:
  --reason STRING            reason for aborting (recorded in the build log)
  --abort-with-success       mark the aborted build as successful
  --skip-git-status-report   skip sending a git status report
  --skip-notifications       skip sending build notifications

```
bitrise-cli build abort BUILD_ID [flags]
```

### Examples

```
  bitrise-cli build abort BUILD_ID --app my-app-id
  bitrise-cli build abort BUILD_ID --app my-app-id --reason "no longer needed"
  bitrise-cli build abort BUILD_ID --app my-app-id --abort-with-success
  bitrise-cli build abort BUILD_ID --app my-app-id --output json
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
      --app string      app ID (or set BITRISE_APP_ID)
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli build](bitrise-cli_build.md)	 - Trigger, list, and inspect builds

