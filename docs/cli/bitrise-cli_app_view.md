## bitrise-cli app view

Show details of a single app

### Synopsis

Show details for a single app identified by its slug.

Argument:
  APP_SLUG           the unique slug of the app (visible in app URLs);
                     falls back to BITRISE_APP_SLUG when omitted

Flags:
  --web              open the app page in the browser instead of printing

```
bitrise-cli app view APP_SLUG [flags]
```

### Examples

```
  bitrise-cli app view stub-app-aaa
  bitrise-cli app view stub-app-aaa --output json
  bitrise-cli app view stub-app-aaa --web
  BITRISE_APP_SLUG=stub-app-aaa bitrise-cli app view
```

### Options

```
  -h, --help   help for view
      --web    open the app page in the browser
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli app](bitrise-cli_app.md)	 - List, inspect, and manage apps (also: project)

