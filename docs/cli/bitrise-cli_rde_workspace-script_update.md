## bitrise-cli rde workspace-script update

Change the workspace script, its state or its variables

### Synopsis

Change the workspace script: the script every session in the workspace
runs first at each boot (see 'bitrise-cli rde workspace-script --help').

Only what the flags name changes; everything else is kept. The CLI reads the
current script (secret values included), applies the changes and saves the
whole script back. Protected variables you don't touch keep their stored
value.

Saving changes what every member's sessions run at their next boot (while
the script is enabled). Test a change first: save it disabled, create a test
session with 'bitrise-cli rde session create NAME --workspace-script-test',
check the "[workspace]" lines in its System log, then --enable.

This suits keeping the script in git and deploying it from CI with
--script-file and a personal access token of someone with the workspace's
Usage permission — Workspace API Tokens cannot edit the workspace script.

--set-var, --set-secret and --set-protected add a variable or replace an
existing one's value and visibility (plain, secret, protected).

Pass secret and protected values on stdin: --set-secret KEY=- (or
--set-protected KEY=-) reads the value from stdin, with one trailing newline
trimmed, and keeps it out of shell history and process arguments. A value
given inline is visible in both. Only one thing can read stdin per call: a
single KEY=- value or --script-file -.

```
bitrise-cli rde workspace-script update [flags]
```

### Examples

```
  bitrise-cli rde workspace-script update --script-file workspace-script.sh
  cat workspace-script.sh | bitrise-cli rde workspace-script update --script-file -
  bitrise-cli rde workspace-script update --set-var REGION=eu
  # Secrets from stdin, not the command line (e.g. from a CI secret env var).
  printf '%s' "$AGENT_TOKEN" | bitrise-cli rde workspace-script update --set-secret AGENT_TOKEN=-
  bitrise-cli rde workspace-script update --remove-var OLD_FLAG
  # Try the draft on a test session, then turn it on for everyone.
  bitrise-cli rde workspace-script update --script-file workspace-script.sh --disable
  bitrise-cli rde session create try-it --workspace-script-test --template TEMPLATE_ID
  bitrise-cli rde workspace-script update --enable
```

### Options

```
      --disable                     disable the script (draft): only sessions created with --workspace-script-test run it
      --enable                      enable the script: every session runs it at its next boot
  -h, --help                        help for update
      --remove-var stringArray      remove the variable KEY (repeatable)
      --script-file string          path to the bash script to save ('-' reads stdin)
      --set-protected stringArray   set a protected variable as KEY=VALUE: encrypted and write-only, never shown again; KEY=- reads the value from stdin (recommended) (repeatable)
      --set-secret stringArray      set a secret variable as KEY=VALUE: encrypted at rest, hidden unless viewed with --show-secrets; KEY=- reads the value from stdin (recommended) (repeatable)
      --set-var stringArray         set a plain variable as KEY=VALUE (repeatable)
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

* [bitrise-cli rde workspace-script](bitrise-cli_rde_workspace-script.md)	 - View and edit the script every session in the workspace boots with

