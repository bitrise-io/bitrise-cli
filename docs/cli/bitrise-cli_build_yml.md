## bitrise-cli build yml

Print the bitrise.yml a specific build ran with

### Synopsis

Print the bitrise.yml configuration that a specific build ran with.

This is a shortcut for 'bitrise-cli yml get --build BUILD_SLUG'.

Required:
  --app SLUG    app slug (or BITRISE_APP_SLUG env var)

```
bitrise-cli build yml BUILD_SLUG [flags]
```

### Examples

```
  bitrise-cli build yml abc123 --app my-app-slug
  bitrise-cli build yml abc123 --app my-app-slug --output json
```

### Options

```
  -h, --help   help for yml
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

