## bitrise-cli rde workspace-script

View and edit the script every session in the workspace boots with

### Synopsis

View and edit the workspace script: one bash script per workspace that
every session boot runs first — every session (from a template, ad hoc, from
a warm pool, device sessions), every boot (including restore and rebuild),
on macOS and Linux. It runs as the session user from $HOME via 'bash -l',
with its variables in the environment, and its output goes to the session's
System log, each line prefixed "[workspace] ".

The template's lifecycle hooks wait until the workspace script
exits, at most 10 minutes; after that they start anyway and the
script keeps running (it is never killed). A line '# rde: no-wait-limit' in
the script lifts the limit. Start long-running processes in the background
('my-agent &') so the script can exit.

Disabled is draft mode, not off: the script stays saved and runs only on
test sessions — personal sessions created with 'bitrise-cli rde session
create NAME --workspace-script-test'. Test the script that way before
enabling it: once enabled, every member's sessions run it at their next
boot.

Variables are plain (value shown), secret (encrypted at rest, shown only
with --show-secrets) or protected (encrypted and write-only: never shown,
only whether a value is set). Keys starting with RDE_ are reserved.

Only people with the workspace's Usage permission (workspace owners and
billing managers) can view or edit the script, with a personal access token
— Workspace API Tokens are refused. The feature must be enabled for the
workspace by Bitrise.

### Examples

```
  bitrise-cli rde workspace-script view
  bitrise-cli rde workspace-script update --script-file workspace-script.sh
  bitrise-cli rde session create try-it --workspace-script-test --template TEMPLATE_ID
  bitrise-cli rde workspace-script update --enable
```

### Options

```
  -h, --help   help for workspace-script
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
* [bitrise-cli rde workspace-script update](bitrise-cli_rde_workspace-script_update.md)	 - Change the workspace script, its state or its variables
* [bitrise-cli rde workspace-script view](bitrise-cli_rde_workspace-script_view.md)	 - Show the workspace script, its state and its variables

