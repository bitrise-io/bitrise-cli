## bitrise-cli rde warm-pool update

Update a warm pool's name, count or configuration

### Synopsis

Update a warm pool. Only the flags you pass are sent; everything else is
left as it is.

Session inputs and feature flags are replaced as a whole: passing any
--input / --secret-input / --saved-input flag replaces the pool's entire
input list with the ones given, and any --feature-flag replaces the enabled
flags. Use --clear-inputs / --clear-feature-flags to empty a list.
--stack, --machine-type and --cluster set an override; pass an empty value
("") to remove one and fall back to the template's.

Changing the configuration invalidates the pool's current inventory: the
backend terminates the existing warm sessions and boots new ones from the
updated configuration. Changing only --name or --count does not. For the
count alone, 'rde warm-pool set-count' is the shorter spelling.

```
bitrise-cli rde warm-pool update WARM_POOL_ID [flags]
```

### Examples

```
  bitrise-cli rde warm-pool update WARM_POOL_ID --name ios-devs-eu --count 4
  # Replace the inputs (all of them) and switch to a bigger machine.
  bitrise-cli rde warm-pool update ios-devs --secret-input GITHUB_TOKEN=ghp_yyy --machine-type g2.mac.m2pro.12c-32g
  # Drop the stack override so the template's stack applies again.
  bitrise-cli rde warm-pool update ios-devs --stack ""
  bitrise-cli rde warm-pool update ios-devs --clear-feature-flags
```

### Options

```
      --clear-feature-flags        disable every feature flag
      --clear-inputs               remove every stored session input
      --cluster string             cluster override; "" removes the override
      --count int                  new desired count of warm sessions; 0 drains the pool but keeps it as a preset
      --feature-flag stringArray   feature flag to enable on the warm sessions (repeatable; replaces the enabled flags as a whole)
  -h, --help                       help for update
      --input stringArray          session input as key=value (repeatable)
      --machine-type string        machine type override; "" removes the override
      --name string                new name
      --saved-input stringArray    session input as key=savedInputID — uses one of your stored saved-input values (repeatable; user pools only)
      --secret-input stringArray   session input as key=value, stored as a secret at rest (repeatable; the value is visible in shell history and process args — prefer --saved-input on a user pool)
      --stack string               stack ID override; "" removes the override
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

