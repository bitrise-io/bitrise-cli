## bitrise-cli step search

Find steps by name, description, or tags

### Synopsis

Find steps for use in workflows or step bundles.

Returns only the latest, non-deprecated version of each matching step.

Arguments:
  QUERY   phrase to search for (e.g. "clone", "npm", "deploy")

Filters:
  --category VALUE    filter by category; may be repeated
  --maintainer VALUE  filter by maintainer; may be repeated

Valid categories:
  build, code-sign, test, deploy, notification, access-control,
  artifact-info, installer, dependency, utility

Valid maintainers:
  bitrise   official Bitrise steps
  verified  verified community steps
  community all community steps

```
bitrise-cli step search QUERY [flags]
```

### Examples

```
  bitrise-cli step search clone
  bitrise-cli step search deploy --category deploy --maintainer bitrise
  bitrise-cli step search npm --output json
```

### Options

```
      --category stringArray     filter by category (may be repeated)
  -h, --help                     help for search
      --maintainer stringArray   filter by maintainer: bitrise, verified, community (may be repeated)
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

