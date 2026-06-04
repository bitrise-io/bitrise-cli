## bitrise-cli step inputs

List inputs of a step version

### Synopsis

List the inputs (and their defaults) for a given step version.

STEP_REF must include an exact version: step_id@version
For custom step sources: step_lib_source::step_id@version

Arguments:
  STEP_REF   step reference, e.g. git-clone@8.3.1

```
bitrise-cli step inputs STEP_REF [flags]
```

### Examples

```
  bitrise-cli step inputs git-clone@8.3.1
  bitrise-cli step inputs git-clone@8.3.1 --output json
```

### Options

```
  -h, --help   help for inputs
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli step](bitrise-cli_step.md)	 - Search steps and inspect their inputs

