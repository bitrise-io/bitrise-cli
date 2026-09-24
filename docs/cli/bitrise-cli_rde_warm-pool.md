## bitrise-cli rde warm-pool

Keep pre-booted sessions ready to claim (warm pools)

### Synopsis

Keep pre-booted sessions ready to claim.

A warm pool is a stored session configuration — a template, its session
input values and feature flags, and optional stack / machine type / cluster
overrides — plus an owner and a pool size. The RDE backend keeps that
many sessions of the configuration booted and idle ("warm sessions"), so
'rde session create --warm-pool POOL' hands one out instantly (the session's
warm state is "claimed") instead of booting a VM. When none is available the
session is created from the pool's configuration ("cold"), so a pool with a
pool size of 0 still works as a configuration preset.

Owner: a "user" pool is private to its creator and may reference the
creator's saved inputs; a "workspace" pool is shared with every member,
reachable by Workspace API Tokens, and stores every input as a plain value.
Only workspace pools can back preview links ('rde preview-link create
--warm-pool').

Warm sessions cost machine time while idle. Set the pool size to what
demand needs and scale it with 'rde warm-pool set-size POOL N' — the
intended way to warm a pool for business hours is a cron job that sets the
size in the morning and back to 0 in the evening (a Workspace API Token
works for workspace pools). Secret input values are never shown; 'view'
reports each secret key as (hidden).

Commands that take a WARM_POOL_ID also accept a pool name — it's resolved to
an ID for you. Names aren't unique, so if more than one pool shares the name
the command errors and lists the candidate IDs to pick from.

```
bitrise-cli rde warm-pool [flags]
```

### Examples

```
  bitrise-cli rde warm-pool list
  bitrise-cli rde warm-pool create ios-devs --template TEMPLATE_ID --size 2 --owner workspace --secret-input GITHUB_TOKEN=ghp_xxx
  bitrise-cli rde warm-pool set-size ios-devs 0     # drain for the night
  bitrise-cli rde session create dev --warm-pool ios-devs
```

### Options

```
  -h, --help   help for warm-pool
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

* [bitrise-cli rde](bitrise-cli_rde.md)	 - Manage Bitrise Remote Dev Environments (sessions, templates, …)
* [bitrise-cli rde warm-pool create](bitrise-cli_rde_warm-pool_create.md)	 - Create a warm pool
* [bitrise-cli rde warm-pool delete](bitrise-cli_rde_warm-pool_delete.md)	 - Delete a warm pool and terminate its warm sessions
* [bitrise-cli rde warm-pool list](bitrise-cli_rde_warm-pool_list.md)	 - List warm pools in the workspace
* [bitrise-cli rde warm-pool set-size](bitrise-cli_rde_warm-pool_set-size.md)	 - Set a pool's size: how many warm sessions it keeps booted
* [bitrise-cli rde warm-pool update](bitrise-cli_rde_warm-pool_update.md)	 - Update a warm pool's name, count or configuration
* [bitrise-cli rde warm-pool view](bitrise-cli_rde_warm-pool_view.md)	 - Show a warm pool's status, configuration and warm sessions

