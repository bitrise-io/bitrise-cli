## bitrise-cli yml update

Upload a new bitrise.yml to Bitrise

### Synopsis

Upload a new bitrise.yml configuration to Bitrise for an app.

Reads from --file if provided, otherwise reads from stdin.

Note: if the app is configured to read its bitrise.yml from the repository,
this command succeeds but the change will not affect builds.

Required:
  --app ID      app ID (or BITRISE_APP_ID env var)

```
bitrise-cli yml update [flags]
```

### Examples

```
  bitrise-cli yml update --app my-app-id --file bitrise.yml
  cat bitrise.yml | bitrise-cli yml update --app my-app-id
  bitrise-cli yml update --app my-app-id < bitrise.yml
```

### Options

```
  -f, --file string   path to the bitrise.yml file (reads from stdin if omitted)
  -h, --help          help for update
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

* [bitrise-cli yml](bitrise-cli_yml.md)	 - Get, update, or validate the bitrise.yml stored on Bitrise

