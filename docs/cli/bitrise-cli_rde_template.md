## bitrise-cli rde template

List and inspect RDE templates

### Synopsis

List and inspect RDE templates.

Commands that take a TEMPLATE_ID also accept a template name — it's resolved
to an ID for you. Names aren't unique, so if more than one template shares the
name the command errors and lists the candidate IDs to pick from.

```
bitrise-cli rde template [flags]
```

### Options

```
  -h, --help   help for template
```

### Options inherited from parent commands

```
      --no-color           disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string      output format: human|json (default "human")
  -q, --quiet              suppress non-error diagnostic messages
      --theme string       color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
      --workspace string   workspace slug (or set BITRISE_WORKSPACE_ID; defaults to default_workspace_slug)
```

### SEE ALSO

* [bitrise-cli rde](bitrise-cli_rde.md)	 - Manage Bitrise Remote Dev Environments (sessions, templates, …)
* [bitrise-cli rde template create](bitrise-cli_rde_template_create.md)	 - Create a new RDE template from a JSON spec file
* [bitrise-cli rde template delete](bitrise-cli_rde_template_delete.md)	 - Delete an RDE template
* [bitrise-cli rde template list](bitrise-cli_rde_template_list.md)	 - List RDE templates in the workspace
* [bitrise-cli rde template update](bitrise-cli_rde_template_update.md)	 - Update an existing RDE template from a JSON spec file
* [bitrise-cli rde template view](bitrise-cli_rde_template_view.md)	 - Show details of a single template

