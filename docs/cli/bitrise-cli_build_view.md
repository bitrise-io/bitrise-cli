## bitrise-cli build view

Show details of a single build

### Synopsis

Show details for a single build identified by its build ID.

Required flags:
  --app ID           (or BITRISE_APP_ID env var)

Argument:
  BUILD_ID           the unique ID of the build (visible in build URLs)

Flags:
  --web              open the build page in the browser instead of printing

```
bitrise-cli build view BUILD_ID [flags]
```

### Examples

```
  bitrise-cli build view --app my-app-id stub-build-aaa
  bitrise-cli build view --app my-app-id stub-build-aaa --output json
  bitrise-cli build view --app my-app-id stub-build-aaa --web
```

### Options

```
  -h, --help   help for view
      --web    open the build page in the browser
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

