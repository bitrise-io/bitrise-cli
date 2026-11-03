## bitrise-cli rde pool list

List pools in the workspace

### Synopsis

List the workspace's pools. Pass --template to see only the pools
of one template (by ID or name).

--all is the read-only cost view of the same pools, the one 'rde usage'
shows; it requires the workspace's billing-data permission.

READY and WARMING are the pool's current inventory; SIZE is the pool's warm
sessions — the count the backend keeps booted. IN USE/MAX is how many sessions from the
pool are in use right now against its max sessions ("3/10"; a bare count
when the pool has no limit): "full" means a claim that finds no warm
session, a restore or a start is refused until a session ends; "over max"
means the max was lowered below the number in use. STATUS is "ok", or the problem 'view' explains:
"config error" (the stored configuration no longer builds — the pool creates
nothing until it is fixed), "paused" (backing off after repeated failures)
or "error" (the last machine creation failed).

```
bitrise-cli rde pool list [flags]
```

### Examples

```
  bitrise-cli rde pool list
  bitrise-cli rde pool list --template TEMPLATE_ID
  bitrise-cli rde pool list --all --output json | jq '.items[] | {name, pool_size, ready: .status.ready}'
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

* [bitrise-cli rde pool](bitrise-cli_rde_pool.md)	 - Keep pre-booted sessions ready to claim (pools)

