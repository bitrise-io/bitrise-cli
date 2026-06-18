## bitrise-cli rde template delete

Delete an RDE template

### Synopsis

Delete an RDE template (soft-delete server-side). Existing sessions
created from this template keep working — they reference a snapshot — but
the template can no longer be selected for new sessions.

```
bitrise-cli rde template delete TEMPLATE_ID [flags]
```

### Options

```
  -h, --help   help for delete
```

### Options inherited from parent commands

```
      --no-color           disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string      output format: human|json (default "human")
  -q, --quiet              suppress non-error diagnostic messages
      --theme string       color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
      --workspace string   workspace ID (or set BITRISE_WORKSPACE_ID or default_workspace_id; auto-detected if you have exactly one workspace)
```

### SEE ALSO

* [bitrise-cli rde template](bitrise-cli_rde_template.md)	 - List and inspect RDE templates

