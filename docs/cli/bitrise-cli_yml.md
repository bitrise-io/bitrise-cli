## bitrise-cli yml

Get, update, or validate the bitrise.yml stored on Bitrise

### Synopsis

Manage the bitrise.yml configuration stored on Bitrise.

Running 'bitrise-cli yml' without a subcommand defaults to 'yml get'.

Subcommands operate on the YAML stored server-side. If your project stores
bitrise.yml in the repository (version-controlled mode), get and update
commands still work, but uploaded changes will not affect builds.

```
bitrise-cli yml [flags]
```

### Options

```
      --app string   app slug (also accepted as --project; or set BITRISE_APP_SLUG)
  -h, --help         help for yml
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli](bitrise-cli.md)	 - Bitrise platform CLI
* [bitrise-cli yml get](bitrise-cli_yml_get.md)	 - Print the bitrise.yml stored on Bitrise
* [bitrise-cli yml update](bitrise-cli_yml_update.md)	 - Upload a new bitrise.yml to Bitrise
* [bitrise-cli yml validate](bitrise-cli_yml_validate.md)	 - Validate a bitrise.yml file

