## bitrise-cli rde warm-pool set-size

Set a pool's size: how many warm sessions it keeps booted

### Synopsis

Set a warm pool's pool size — how many sessions of its configuration
the RDE backend keeps booted and idle. Raising it boots sessions; lowering it
terminates surplus warm sessions (claimed sessions are untouched). 0 drains
the pool but keeps it usable as a configuration preset: 'rde session create
--warm-pool' then creates sessions on demand from its configuration.

This is the knob for scaling a pool to business hours: run it from a cron job
in the morning and again with 0 in the evening. A Workspace API Token works
for workspace pools, so the job does not need a personal token.

```
bitrise-cli rde warm-pool set-size WARM_POOL_ID SIZE [flags]
```

### Examples

```
  bitrise-cli rde warm-pool set-size ios-devs 3
  # Cron: warm up at 08:00 on weekdays, drain at 19:00.
  0 8  * * 1-5  BITRISE_TOKEN=bitwat_… bitrise-cli rde warm-pool set-size WARM_POOL_ID 3 --workspace WORKSPACE_ID -q
  0 19 * * 1-5  BITRISE_TOKEN=bitwat_… bitrise-cli rde warm-pool set-size WARM_POOL_ID 0 --workspace WORKSPACE_ID -q
```

### Options

```
  -h, --help   help for set-size
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

