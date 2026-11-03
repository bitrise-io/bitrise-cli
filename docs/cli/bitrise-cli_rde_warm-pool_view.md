## bitrise-cli rde warm-pool view

Show a warm pool's status, limits, configuration and warm sessions

### Synopsis

Show a warm pool: its live status (ready / warming inventory, sessions in
use against its max, lifetime claimed and cold counters, refusals in the
last 7 days, errors), the stored configuration with secret input values
hidden, and the warm sessions currently in the pool.

A growing cold count means sessions had to be created on demand because no
warm session was available — the warm sessions count is too low for the demand. A
growing refused count means the pool was full more often than its users can
bear — the max sessions is too low for the demand.

```
bitrise-cli rde warm-pool view WARM_POOL_ID [flags]
```

### Examples

```
  bitrise-cli rde warm-pool view WARM_POOL_ID
  bitrise-cli rde warm-pool view ios-devs --output json | jq .status
```

### Options

```
  -h, --help   help for view
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

