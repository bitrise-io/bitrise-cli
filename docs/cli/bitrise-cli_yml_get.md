## bitrise-cli yml get

Print the bitrise.yml stored on Bitrise

### Synopsis

Print the bitrise.yml configuration stored on Bitrise for an app.

When --build is provided, prints the bitrise.yml that a specific build ran with
instead of the app's current stored configuration.

Required:
  --app ID      app ID (or BITRISE_APP_ID env var)

Optional:
  --build ID    print the yml used for this specific build

```
bitrise-cli yml get [flags]
```

### Examples

```
  bitrise-cli yml get --app my-app-id
  bitrise-cli yml get --app my-app-id --build abc123
  bitrise-cli yml get --app my-app-id --output json
  BITRISE_APP_ID=my-app-id bitrise-cli yml get
```

### Options

```
      --build string   build ID to retrieve the yml for
  -h, --help           help for get
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

