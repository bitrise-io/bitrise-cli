## bitrise-cli build yml

Print the bitrise.yml a specific build ran with

### Synopsis

Print the bitrise.yml configuration that a specific build ran with.

This is a shortcut for 'bitrise-cli yml get --build BUILD_ID'.

Required:
  --app ID      app ID (or BITRISE_APP_ID env var)

```
bitrise-cli build yml BUILD_ID [flags]
```

### Examples

```
  bitrise-cli build yml abc123 --app my-app-id
  bitrise-cli build yml abc123 --app my-app-id --output json
```

### Options

```
  -h, --help   help for yml
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

