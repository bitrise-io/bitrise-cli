## bitrise-cli rde warm-pool retry

Resume a warm pool paused after repeated On create failures

### Synopsis

Resume a warm pool that paused after three consecutive On create failures
('warm-pool view' shows "Setup failed:" and 'list' says "setup failed"): the
machine kept from the last failure is deleted, the failure streak is cleared
and the pool starts warming again.

Do it after fixing what made On create fail — the template's script ('rde
template update') or something outside it (a token, a package mirror). Look at
the kept machine first: the session id on the "Setup failed:" line is a
session whose logs ('rde session logs --stage on-create') and terminal still
work. Any configuration change to the pool or its template resumes warming
the same way, so 'retry' is for "the script was fine, the world was not".

A pool that is not paused this way is returned unchanged.

```
bitrise-cli rde warm-pool retry WARM_POOL_ID [flags]
```

### Examples

```
  bitrise-cli rde warm-pool retry ios-devs
```

### Options

```
  -h, --help   help for retry
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

* [bitrise-cli rde warm-pool](bitrise-cli_rde_warm-pool.md)	 - Keep pre-booted sessions ready to claim (warm pools)

