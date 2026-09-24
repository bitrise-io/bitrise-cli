## bitrise-cli rde warm-pool delete

Delete a warm pool and terminate its warm sessions

### Synopsis

Delete a warm pool. Its warm sessions — the booted, unclaimed ones — are
terminated and removed by the backend; sessions already claimed from the
pool are ordinary sessions and are untouched. Preview links minted with this
pool stop working: every later open fails as an expired link would.

This cannot be undone. Pass --yes to skip the confirmation prompt. To stop
paying for idle machines while keeping the configuration and its links,
set the pool size to 0 instead ('rde warm-pool set-size POOL 0').

```
bitrise-cli rde warm-pool delete WARM_POOL_ID [flags]
```

### Examples

```
  bitrise-cli rde warm-pool delete WARM_POOL_ID
  bitrise-cli rde warm-pool delete ios-devs --yes
```

### Options

```
  -h, --help   help for delete
      --yes    skip the confirmation prompt
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

