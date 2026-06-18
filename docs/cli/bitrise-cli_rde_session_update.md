## bitrise-cli rde session update

Update a session's name, description, or auto-terminate duration

```
bitrise-cli rde session update SESSION_ID [flags]
```

### Examples

```
  bitrise-cli rde session update SESSION_ID --name new-name
  bitrise-cli rde session update SESSION_ID --auto-terminate-minutes 0
```

### Options

```
      --auto-terminate-minutes int   auto-terminate duration in minutes; 0 disables. Resets the deadline to now + minutes.
      --description string           new session description
  -h, --help                         help for update
      --name string                  new session name
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

* [bitrise-cli rde session](bitrise-cli_rde_session.md)	 - Create, list, inspect, and manage RDE sessions

