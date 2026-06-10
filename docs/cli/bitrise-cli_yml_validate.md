## bitrise-cli yml validate

Validate a bitrise.yml file

### Synopsis

Validate a bitrise.yml against the Bitrise API.

Reads from --file if provided, otherwise reads from stdin.

When --app is provided (or BITRISE_APP_ID is set), validation uses
app-specific settings (available stacks, machine types, license pools).
Without an app ID, only the schema is validated.

Exit codes:
  0   valid (no errors; warnings do not affect the exit code)
  1   invalid (at least one error)

```
bitrise-cli yml validate [flags]
```

### Examples

```
  bitrise-cli yml validate --file bitrise.yml
  bitrise-cli yml validate --file bitrise.yml --app my-app-id
  cat bitrise.yml | bitrise-cli yml validate
  bitrise-cli yml validate --file bitrise.yml --output json
```

### Options

```
  -f, --file string   path to the bitrise.yml file (reads from stdin if omitted)
  -h, --help          help for validate
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

* [bitrise-cli yml](bitrise-cli_yml.md)	 - Get, update, or validate the bitrise.yml stored on Bitrise

