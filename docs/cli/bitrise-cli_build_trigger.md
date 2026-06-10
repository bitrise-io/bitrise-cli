## bitrise-cli build trigger

Start a new build

### Synopsis

Start a new build on the given app.

Required flags:
  --app ID           (or BITRISE_APP_ID env var)

Optional flags:
  --workflow ID          workflow ID (mutually exclusive with --pipeline); Bitrise
                         selects the appropriate workflow from the trigger map if omitted
  --pipeline ID          pipeline ID (mutually exclusive with --workflow)
  --branch BRANCH        branch to build (default "main" for branch builds)
  --branch-dest BRANCH   target branch for pull-request builds
  --tag TAG              tag to build
  --commit-hash HASH     commit hash to build from
  --commit-message MSG   commit message to record
  --pull-request-id ID   pull request ID for PR builds
  --priority N           build priority (-1 = low, 0 = normal, 1 = high)
  --env JSON             environment variables as a JSON object, e.g. '{"KEY":"value"}'
  --wait                 wait for the build to finish without streaming logs; exits 0 on
                         success, 1 on failure. With --output json the final build record
                         is written to stdout.
  --watch                stream build logs until the build finishes; exits 0 on success,
                         1 on failure. With --output json logs go to stderr and the final
                         build record is written to stdout.
  --interval DURATION    polling interval when --wait or --watch is active (default 3s)

```
bitrise-cli build trigger [flags]
```

### Examples

```
  bitrise-cli build trigger --app my-app-id --workflow primary
  bitrise-cli build trigger --app my-app-id --workflow deploy --branch release/1.2 --output json
  bitrise-cli build trigger --app my-app-id --pipeline my-pipeline --branch main
  bitrise-cli build trigger --app my-app-id --workflow primary --tag v1.2.3
  bitrise-cli build trigger --app my-app-id --workflow primary --env '{"MY_VAR":"hello","OTHER":"world"}'
  bitrise-cli build trigger --app my-app-id --workflow primary --wait
  bitrise-cli build trigger --app my-app-id --workflow primary --watch
  bitrise-cli build trigger --app my-app-id --workflow primary --watch --output json
```

### Options

```
      --branch string           branch to build (default "main" for branch builds)
      --branch-dest string      target branch for pull-request builds
      --commit-hash string      commit hash to build
      --commit-message string   commit message to record
      --env string              environment variables as a JSON object, e.g. '{"KEY":"value"}'
  -h, --help                    help for trigger
      --interval duration       polling interval when --wait or --watch is active (default 3s)
      --pipeline string         pipeline ID to trigger (mutually exclusive with --workflow)
      --priority int            build priority (-1 = low, 0 = normal, 1 = high)
      --pull-request-id int     pull request ID for PR builds
      --tag string              tag to build
      --wait                    block until the build finishes without streaming logs (exit code reflects build outcome)
      --watch                   stream build logs until the build finishes (exit code reflects build outcome)
      --workflow string         workflow ID to trigger (mutually exclusive with --pipeline)
```

### Options inherited from parent commands

```
      --app string      app ID (also accepted as --project; or set BITRISE_APP_ID)
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli build](bitrise-cli_build.md)	 - Trigger, list, and inspect builds

