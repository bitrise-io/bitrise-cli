## bitrise-cli rde session create

Create a new RDE session from a template

### Synopsis

Create a new RDE session from a template.

NAME is a human-readable label for the session; you can use it in place of the
session ID in later commands (view, terminate, …) as long as it stays unique.

Provide session input values via --input (one --input per key), --secret-input
(value stored as secret-at-rest), or --saved-input (reference an existing saved
input by ID). Use --map-saved-inputs to auto-fill any session input key that
matches a saved input the user already has.

For secret values, prefer storing them once with 'rde saved-input create
--value-stdin --secret' and referencing them by ID via --saved-input. A value
passed inline with --secret-input ends up in your shell history and in the
process arguments (readable by other users via 'ps'); marking it secret only
governs how the backend stores the value, not how it reaches the CLI.

Example values:
  --input key=value
  --saved-input session-key=SAVED_INPUT_ID   # secret stored ahead of time
  --secret-input api-key=VALUE               # inline; avoid for real secrets

```
bitrise-cli rde session create NAME [flags]
```

### Examples

```
  bitrise-cli rde session create dev --template TEMPLATE_ID
  bitrise-cli rde session create dev --template TEMPLATE_ID --input repo=my-app
  # Keep secrets off the command line: store once, then reference by ID.
  echo -n "ghp_xxx" | bitrise-cli rde saved-input create --key gh-token --value-stdin --secret
  bitrise-cli rde session create dev --template TEMPLATE_ID --saved-input gh-token=SAVED_INPUT_ID
  bitrise-cli rde session create dev --template TEMPLATE_ID --map-saved-inputs
```

### Options

```
      --ai-prompt string             initial AI prompt passed to Claude Code on session start
      --auto-terminate-minutes int   minutes until auto-termination; 0 disables; omitted uses backend default (~5 days)
      --cluster string               target cluster name (use 'rde machine-type list --image NAME' to find candidates when the image + machine type combo is ambiguous)
      --description string           session description
      --feature-flag stringArray     name of a feature flag to enable on the session (repeatable)
  -h, --help                         help for create
      --input stringArray            session input as key=value (repeatable)
      --map-saved-inputs             auto-fill template session inputs from the user's saved inputs (matched by key)
      --saved-input stringArray      session input as key=savedInputID — uses a stored saved-input value (repeatable)
      --secret-input stringArray     session input as key=value, stored as a secret at rest (repeatable; the value is visible in shell history and process args — prefer --saved-input)
      --template string              template ID or name to create the session from (required)
      --wait                         wait until the session leaves provisioning (running, failed, …) before returning; exits 1 if the final status isn't running
      --wait-timeout duration        max time to wait when --wait is set (uses Go duration syntax: 30s, 5m, 1h) (default 10m0s)
```

### Options inherited from parent commands

```
      --no-color           disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string      output format: human|json (default "human")
  -q, --quiet              suppress non-error diagnostic messages
      --theme string       color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
      --workspace string   workspace ID (or set BITRISE_WORKSPACE_ID; defaults to default_workspace_id)
```

### SEE ALSO

* [bitrise-cli rde session](bitrise-cli_rde_session.md)	 - Create, list, inspect, and manage RDE sessions

