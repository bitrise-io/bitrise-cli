## bitrise-cli rde warm-pool create

Create a warm pool

### Synopsis

Create a warm pool: a stored session configuration the RDE backend keeps
--size sessions of booted and idle, ready to be claimed with 'rde session
create --warm-pool NAME'.

NAME is a human-readable label for the pool; you can use it in place of the
pool ID in later commands as long as it stays unique. --template names the
template (by ID or name) the warm sessions are created from; the other
configuration flags are those of 'rde session create' and are validated the
same way — a pool can only store what a session request could send.

--size is how many warm sessions to keep booted. It defaults to 0: the pool
is created as an inert configuration preset and costs nothing until you
raise the size with 'rde warm-pool set-size'. Every warm session costs
machine time while idle.

The pool belongs to the workspace: every member sees, claims from and
manages it, a Workspace API Token can create and claim from it, and it can
back a preview link. A pool carries no personal state: give every template
session input as a value (--input / --secret-input); saved inputs are
personal and cannot be referenced from a pool.

Provide session input values via --input (one --input per key) or
--secret-input (stored as secret-at-rest). A value passed inline with
--secret-input ends up in your shell history and process arguments.

--stack, --machine-type and --cluster override the template's defaults for
the warm sessions; omit them to use the template's.

The device the warm sessions boot follows 'rde session create': omit the
device flags to boot the template's declared device as is; --device-model,
--device-os-version and --device-system-image without --device-platform tweak
that device per field; with --device-platform the flags are the complete
device to boot; --no-device boots none. A claimed session's app artifact is
still given at claim time.

```
bitrise-cli rde warm-pool create NAME [flags]
```

### Examples

```
  bitrise-cli rde warm-pool create ios-devs --template TEMPLATE_ID --size 2
  # Inputs as plain values; a feature flag enabled on every warm session.
  bitrise-cli rde warm-pool create ci-devices --template TEMPLATE_ID --size 3 --secret-input GITHUB_TOKEN=ghp_xxx --feature-flag enable_beta_simulator
  # A configuration preset: nothing booted until 'set-size' raises the count.
  bitrise-cli rde warm-pool create preset --template TEMPLATE_ID --machine-type g2.mac.m2pro.6c-14g
  # The template's simulator, but an iPhone 15 on iOS 17.5; or no device at all.
  bitrise-cli rde warm-pool create ios-17 --template TEMPLATE_ID --device-model "iPhone 15" --device-os-version 17.5
  bitrise-cli rde warm-pool create headless --template TEMPLATE_ID --no-device
```

### Options

```
      --cluster string               target cluster name to override the one resolved from stack + machine type
      --device-model string          device to boot: simctl device type ("iPhone 16") or emulator device profile ("pixel_7") — a screen profile, not that phone's firmware; default: the template's, else the platform default
      --device-os-version string     iOS only: an iOS version ("18.2") or simctl runtime id — anything else is rejected; default: the template's, else newest installed
      --device-platform string       device the warm sessions boot: ios (simulator, macOS stack) or android (emulator, Linux stack); omit with --template to tweak the template's device per field
      --device-system-image string   Android only: the API-level knob — sdkmanager system image package ("system-images;android-34;google_apis;x86_64"); default: the template's, else the platform default
      --feature-flag stringArray     name of a template feature flag to enable on the warm sessions (repeatable)
  -h, --help                         help for create
      --input stringArray            session input as key=value (repeatable)
      --machine-type string          machine type name to override the template's (see 'rde machine-type list --stack STACK_ID')
      --no-device                    boot the warm sessions without the template's device (ignored when the template declares none)
      --secret-input stringArray     session input as key=value, stored as a secret at rest (repeatable; the value is visible in shell history and process args)
      --size int                     how many warm sessions to keep booted; 0 (the default) creates an inert configuration preset
      --stack string                 stack ID to override the template's (see 'rde stack list')
      --template string              template ID or name the warm sessions are created from (required)
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

