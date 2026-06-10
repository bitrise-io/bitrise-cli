## bitrise-cli build watch

Stream logs for a running build

### Synopsis

Stream build logs until the build finishes, then exit with a status
reflecting the build outcome (0 = success, 1 = failed or aborted).

Ctrl-C detaches the CLI without affecting the running build.

Required flags:
  --app ID           (or BITRISE_APP_ID env var)

Argument:
  BUILD_ID           the unique ID of the build

Output:
  human (default)  logs stream as raw text; a header/footer frame them on stderr.
  json             logs stream to stderr and the final build record is written
                   to stdout, so 'build watch ... -o json' is pipeable.

```
bitrise-cli build watch BUILD_ID [flags]
```

### Examples

```
  bitrise-cli build watch --app my-app-id <build-id>
  bitrise-cli build watch --app my-app-id <build-id> --interval 5s
  bitrise-cli build watch --app my-app-id <build-id> --output json
```

### Options

```
  -h, --help                help for watch
      --interval duration   log polling interval (default 3s)
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

