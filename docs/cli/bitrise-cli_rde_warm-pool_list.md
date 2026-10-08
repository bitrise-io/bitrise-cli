## bitrise-cli rde warm-pool list

List warm pools in the workspace

### Synopsis

List the workspace's warm pools. Pass --template to see only the pools
of one template (by ID or name).

--all is the read-only cost view of the same pools, the one 'rde usage'
shows; it requires the workspace's billing-data permission.

READY and WARMING are the pool's current inventory; SIZE is the pool size —
the count the backend keeps booted. STATUS is "ok", or the problem 'view' explains:
"config error" (the stored configuration no longer builds — the pool creates
nothing until it is fixed), "paused" (backing off after repeated failures)
or "error" (the last machine creation failed).

```
bitrise-cli rde warm-pool list [flags]
```

### Examples

```
  bitrise-cli rde warm-pool list
  bitrise-cli rde warm-pool list --template TEMPLATE_ID
  bitrise-cli rde warm-pool list --all --output json | jq '.items[] | {name, pool_size, ready: .status.ready}'
```

### Options

```
      --all               the read-only cost view (requires the billing-data permission)
  -h, --help              help for list
      --template string   only pools of this template (ID or name)
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

