## bitrise-cli rde

Manage Bitrise Remote Dev Environments (sessions, templates, …)

### Synopsis

Manage Bitrise Remote Dev Environments — sessions, templates, saved inputs,
and the machine catalog (images, machine types).

Workspace resolution (highest to lowest precedence):
  --workspace SLUG          flag on the rde command
  BITRISE_WORKSPACE_ID      environment variable
  default_workspace_slug    saved with 'bitrise-cli config set'

Saved inputs are user-scoped — they do not require --workspace.

### Options

```
  -h, --help               help for rde
      --workspace string   workspace slug (or set BITRISE_WORKSPACE_ID; defaults to default_workspace_slug)
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
* [bitrise-cli rde image](bitrise-cli_rde_image.md)	 - List machine images available to the workspace
* [bitrise-cli rde machine-type](bitrise-cli_rde_machine-type.md)	 - List machine types compatible with a given image
* [bitrise-cli rde saved-input](bitrise-cli_rde_saved-input.md)	 - Manage saved inputs (reusable credentials/values)
* [bitrise-cli rde session](bitrise-cli_rde_session.md)	 - Create, list, inspect, and manage RDE sessions
* [bitrise-cli rde template](bitrise-cli_rde_template.md)	 - List and inspect RDE templates

