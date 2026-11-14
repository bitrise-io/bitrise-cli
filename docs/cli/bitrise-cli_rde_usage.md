## bitrise-cli rde usage

Show the workspace's active session and resource usage

### Synopsis

Show a point-in-time snapshot of the workspace's active remote dev sessions:
session counts and vCPU/memory totals split by OS, workspace-wide and per user.

This reports sessions currently consuming resources; it is not a historical or
billing-period report. Requires the workspace's billing-view permission
(workspace owners and billing-managing custom roles), or a Workspace API
Token created with Remote Dev Environments access and a workspace role that
can manage the workspace (Manager, or a custom role with "Modify workspace
settings").

It also shows the workspace's vCPU quota per machine family the plan includes:
the vCPUs in use (warm pool sessions included) out of the plan's quota. A new session cannot
start on a family once its vCPUs would push the use over the quota.

Pass --sessions to also list, under each user, the active sessions behind the
numbers, oldest first: name, status, age, OS, vCPU/memory and ID. With
--output json the list is always there, as users[].sessions. The list is
filled only on a workspace where terminating any session is enabled (it is
enabled per workspace on request); elsewhere it is empty and the report is
otherwise the same.

On such a workspace, whoever can view this report can also terminate any
listed session with 'rde session terminate SESSION_ID' — a way to find and
stop sessions left running. The exception is a session held by a pool (HELD BY
POOL in the list, held_by_warm_pool_id in JSON): a warm session nobody has
claimed yet. Lower the pool's size to remove it ('rde warm-pool set-size').

Pass --os linux or --os macos to look at one OS only, for example when macOS
capacity is the tight one. The per-user table then shows only that OS's
numbers and leaves out users with no active session on it; the session lists
(--sessions) and the quota table show only that OS. The summary lines at the
top stay workspace-wide. A session whose OS could not be determined is on
neither OS. With --output json the same filter applies: users keeps only the
rows with an active session on that OS, each sessions array only that OS's
sessions and compute_quotas only that OS's machine families, while the totals
objects keep their shape (they are split by OS already).

```
bitrise-cli rde usage [flags]
```

### Examples

```
  bitrise-cli rde usage
  bitrise-cli rde usage --sessions
  bitrise-cli rde usage --sessions --os macos
  bitrise-cli rde usage --output json | jq '.totals'
  bitrise-cli rde usage --output json | jq '.compute_quotas'
  # IDs of the sessions created more than 7 days ago, pool-held ones left out
  bitrise-cli rde usage --output json | jq -r '.users[].sessions[]
    | select(.held_by_warm_pool_id == "" and .created_at < (now - 7*86400 | todate)) | .id'
```

### Options

```
  -h, --help        help for usage
      --os string   show one OS only: linux or macos (narrows the per-user table, the session lists and the quota table; applies to --output json too)
      --sessions    also list each user's active sessions, oldest first (--output json always includes them)
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

