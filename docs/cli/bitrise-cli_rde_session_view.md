## bitrise-cli rde session view

Show details of a single session

### Synopsis

Show details of a single session.

Pass --watch to poll the session and re-render on every change until you
hit Ctrl-C — useful while waiting for a session to come up. --watch is
incompatible with --output json (the contract is a single object, not a
stream); use 'session create --wait' or a polling jq loop instead.

```
bitrise-cli rde session view SESSION_ID [flags]
```

### Options

```
  -h, --help                help for view
      --interval duration   polling interval when --watch is set (Go duration syntax: 1s, 500ms, …) (default 3s)
      --watch               poll the session and re-render on every change until Ctrl-C
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

