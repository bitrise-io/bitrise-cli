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
("") to remove one and fall back to the template's. The device flags set the
pool's device override (without --device-platform they tweak the template's
declared device per field; with it they are the complete device);
--clear-device removes the override so the template's device applies as
declared; --no-device / --no-device=false skip or restore the template's
device.

Changing the configuration invalidates the pool's current inventory: the
backend terminates the existing warm sessions and boots new ones from the
updated configuration. Changing only --name or --size does not. For the
count alone, 'rde warm-pool set-size' is the shorter spelling.

```
bitrise-cli rde warm-pool update WARM_POOL_ID [flags]
```

### Examples

```
  bitrise-cli rde warm-pool update WARM_POOL_ID --name ios-devs-eu --size 4
  # Replace the inputs (all of them) and switch to a bigger machine.
  bitrise-cli rde warm-pool update ios-devs --secret-input GITHUB_TOKEN=ghp_yyy --machine-type g2.mac.m2pro.12c-32g
  # Drop the stack override so the template's stack applies again.
  bitrise-cli rde warm-pool update ios-devs --stack ""
  bitrise-cli rde warm-pool update ios-devs --clear-feature-flags
  # Boot an iPhone 15 instead of the template's device; then go back to the template's.
  bitrise-cli rde warm-pool update ios-devs --device-model "iPhone 15"
  bitrise-cli rde warm-pool update ios-devs --clear-device
```

### Options

```
      --clear-device                 remove the pool's device override; the template's device applies as declared
      --clear-feature-flags          disable every feature flag
      --clear-inputs                 remove every stored session input
      --cluster string               cluster override; "" removes the override
      --device-model string          device to boot: simctl device type ("iPhone 16") or emulator device profile ("pixel_7") — a screen profile, not that phone's firmware; default: the template's, else the platform default
      --device-os-version string     iOS only: an iOS version ("18.2") or simctl runtime id — anything else is rejected; default: the template's, else newest installed
      --device-platform string       device the warm sessions boot: ios (simulator, macOS stack) or android (emulator, Linux stack); omit with --template to tweak the template's device per field
      --device-system-image string   Android only: the API-level knob — sdkmanager system image package ("system-images;android-34;google_apis;x86_64"); default: the template's, else the platform default
      --feature-flag stringArray     feature flag to enable on the warm sessions (repeatable; replaces the enabled flags as a whole)
  -h, --help                         help for update
      --input stringArray            session input as key=value (repeatable)
      --machine-type string          machine type override; "" removes the override
      --name string                  new name
      --no-device                    true: the warm sessions boot without the template's device; --no-device=false boots it again
      --saved-input stringArray      session input as key=savedInputID — uses one of your stored saved-input values (repeatable; user pools only)
      --secret-input stringArray     session input as key=value, stored as a secret at rest (repeatable; the value is visible in shell history and process args — prefer --saved-input on a user pool)
      --size int                     new pool size of warm sessions; 0 drains the pool but keeps it as a preset
      --stack string                 stack ID override; "" removes the override
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

