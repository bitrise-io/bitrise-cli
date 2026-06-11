## bitrise-cli rde template update

Update an existing RDE template from a JSON spec file

### Synopsis

Update an existing RDE template from a JSON spec file.

Only fields present in the file are sent. Array fields (template_variables,
session_inputs, feature_flags, workspace_links) replace the server's
existing list wholesale when present — to clear one, include it as [].

Round-trip workflow:

  bitrise-cli rde template view TEMPLATE_ID -o json > template.json
  # edit template.json
  bitrise-cli rde template update TEMPLATE_ID --file template.json

Pass --file - to read the JSON from stdin.

```
bitrise-cli rde template update TEMPLATE_ID [flags]
```

### Options

```
  -f, --file string   path to a JSON spec file (use '-' for stdin)
  -h, --help          help for update
```

### Options inherited from parent commands

```
      --no-color           disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string      output format: human|json (default "human")
  -q, --quiet              suppress non-error diagnostic messages
      --theme string       color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
      --workspace string   workspace ID (or set BITRISE_WORKSPACE_ID; defaults to default_workspace_id)
```

### SEE ALSO

* [bitrise-cli rde template](bitrise-cli_rde_template.md)	 - List and inspect RDE templates

