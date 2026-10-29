## bitrise-cli rde session create

Create a new RDE session

### Synopsis

Create a new RDE session, either from a template or from a bare
stack + machine type (a template-less session, with no warmup/startup scripts
or other template configuration).

NAME is a human-readable label for the session; you can use it in place of the
session ID in later commands (view, terminate, …) as long as it stays unique.

Pass --template to create the session from a template (by ID or name). To
create a session without a template, omit --template and pass both --stack and
--machine-type instead. --stack / --machine-type may also be given alongside
--template to override the template's defaults for this session.

Provide session input values via --input (one --input per key), --secret-input
(value stored as secret-at-rest), or --saved-input (reference an existing saved
input by ID). Use --map-saved-inputs to auto-fill any session input key that
matches a saved input the user already has.

On a template with lifecycle hooks every input belongs to a scope — Shared,
On create, On claim or On restore ('rde template view' shows it next to each
input). A session created directly from the template needs every required
input of every scope; what is missing is listed per scope. A value given
here reaches every scope that defines its key, so one --input (or one saved
input) fills the same key on several scopes; prefix the key with a scope
(--input on_claim:GH_TOKEN=…) to give that scope its own value, which wins
there over an unscoped one. --secret-input marks a typed value secret
(encrypted at rest, hidden in 'session view') on a direct create and on a
claim alike.

For secret values, prefer storing them once with 'rde saved-input create
--value-stdin --secret' and referencing them by ID via --saved-input. A value
passed inline with --secret-input ends up in your shell history and in the
process arguments (readable by other users via 'ps'); marking it secret only
governs how the backend stores the value, not how it reaches the CLI.

Attach arbitrary key=value metadata with --label (repeatable); labels come
back on 'session view' and in 'session list --output json', and sessions
can be filtered by them with 'rde session list --label-selector key=value'.

By default the session is yours (a personal session). Pass --owner workspace
to create a session owned by the workspace itself: every member can see and
manage it, and it is listed by 'rde session list --scope workspace'. A
workspace session carries no personal state — give every template session
input as a value (--input / --secret-input; --saved-input and
--map-saved-inputs are rejected) and leave --ai-prompt unset. With a
Workspace API Token (bitwat_…, e.g. in CI) every session is workspace-owned:
--owner may be omitted or "workspace", never "user".

Want a device on the session? READ THE GUIDE FIRST: 'bitrise-cli rde
device-guide' (then 'rde device-guide ios' or 'android' for the platform you
boot). It covers the readiness contract, connecting, driving the device
efficiently, letting a human watch, recovery, and what never to do.

Boot a virtual device with the session by passing --device-platform ios (an
iOS simulator on a macOS stack) or android (an Android emulator on a dockerless
Android Linux stack). Prefer omitting --stack/--machine-type: the deployment's
known-good pair for the platform applies (the guide says which stacks fit when
you must name one); with --template the template's stack and machine type are
used and must fit the platform. --cluster is never needed with a device.
--device-model is a screen profile (size, density), not that phone's
firmware; --device-system-image picks the Android API level.

A template may declare a device of its own ('rde template view' shows it as
"Device:"). Sessions created from such a template boot that device as
declared — no device flags needed. The template's device is the base: with
--template, --device-model, --device-os-version and --device-system-image may
be given without --device-platform and tweak the template's device per field
(unset fields inherit the template's). Passing --device-platform makes the
flags the complete device to boot: the template's is ignored and unset fields
are the platform defaults. Pass --no-device to create the session without the
template's device. Optionally pre-install an
app with
--artifact-url, or --artifact-url-stdin to read the URL from stdin: a signed
(pre-authenticated) download URL is a bearer credential, and a value passed
inline ends up in your shell history and in the process arguments (readable
by other users via 'ps'). "running" does not mean the device is usable —
'session view' shows the device state; wait for "ready" (--wait does so for
you when a device was requested) and touch nothing on the VM while it is
"booting". On a template with lifecycle hooks "running" does not mean the
setup is done either: the machine is up before On create (the old warmup)
has finished, and exec, ssh, upload, download and the terminal are refused
until On create has finished ("Session is setting up: On create is still
running. Access opens when it finishes."). 'session view' shows the
lifecycle phase; --wait returns only once it is "ready" (exit 1 on
setup_failed / personalization_failed), so "create --wait && session exec"
is safe on both kinds of template. A "failed" device is not always unusable: 'session view' prints the
device notes, and the guide says which failures leave the device drivable.

Warm pools: pass --warm-pool to claim a session from a warm pool ('rde
warm-pool list') instead of describing one. A booted, idle warm session is
handed out instantly (the session's warm state is "claimed"); when none is
available one is created from the pool's stored configuration ("cold"). The
pool fixes the environment, so --template, --stack, --machine-type,
--feature-flag, --cluster and the device flags are rejected with it — change
the pool instead. What a claim adds is personal: on a template with
lifecycle hooks the input flags give the template's On claim and On restore
inputs ('rde template view' marks them "(on claim)" / "(on restore)"; a
Shared or On create key is refused — those were set on the pool),
--map-saved-inputs fills them from your saved inputs, and --ai-prompt is
accepted — the last two on a personal claim only; a claim kept for the
workspace and a Workspace API Token send plain values, and a missing
required On claim or On restore input fails the claim. NAME, --description,
--label, --auto-terminate-minutes and (on a device pool) the artifact flags
apply too. The claimed session is yours by default; --owner workspace keeps
it the workspace's. It comes back with "Setup: personalizing" while the
platform's System setup and On claim run with your inputs; the On restore
values wait for the first restore.

Example values:
  --input key=value
  --input on_restore:key=value               # this scope only
  --saved-input session-key=SAVED_INPUT_ID   # secret stored ahead of time
  --secret-input api-key=VALUE               # inline; avoid for real secrets

```
bitrise-cli rde session create NAME [flags]
```

### Examples

```
  bitrise-cli rde session create dev --template TEMPLATE_ID
  bitrise-cli rde session create dev --template TEMPLATE_ID --input repo=my-app
  # Template-less: pick a stack and machine type directly.
  bitrise-cli rde session create dev --stack osx-xcode-16.0.x-edge --machine-type g2.mac.m2pro.6c-14g
  # Keep secrets off the command line: store once, then reference by ID.
  echo -n "ghp_xxx" | bitrise-cli rde saved-input create --key gh-token --value-stdin --secret
  bitrise-cli rde session create dev --template TEMPLATE_ID --saved-input gh-token=SAVED_INPUT_ID
  bitrise-cli rde session create dev --template TEMPLATE_ID --map-saved-inputs
  # A session owned by the workspace (shared with every member); inputs as plain values.
  bitrise-cli rde session create ci-smoke --template TEMPLATE_ID --owner workspace --secret-input GITHUB_TOKEN=ghp_xxx
  # Boot an iOS simulator with the session (stack/machine type default to the platform's).
  bitrise-cli rde device-guide ios     # read first: readiness, connecting, driving, do-nots
  bitrise-cli rde session create ios-check --device-platform ios --device-model "iPhone 16" --device-os-version 18.2
  bitrise-cli rde session create android-check --device-platform android --artifact-url https://…/app.apk
  # From a template that declares a device: boot it as declared, override one field, or skip it.
  bitrise-cli rde session create ios-check --template TEMPLATE_ID
  bitrise-cli rde session create ios-check --template TEMPLATE_ID --device-model "iPhone 15"
  bitrise-cli rde session create no-sim --template TEMPLATE_ID --no-device
  # Keep a signed artifact URL out of shell history and process args: read it from a file.
  bitrise-cli rde session create android-check --device-platform android --artifact-url-stdin < artifact-url.txt
  # Claim a pre-booted session from a warm pool (see 'rde warm-pool list').
  bitrise-cli rde session create dev --warm-pool WARM_POOL_ID
```

### Options

```
      --ai-prompt string             initial AI prompt passed to Claude Code on session start
      --artifact-name string         display name of the app installed from --artifact-url / --artifact-url-stdin
      --artifact-url string          app build to install once the device is ready: absolute http(s) URL of a zipped simulator .app (iOS) or an .apk (Android); requires --device-platform or a --template that declares a device (a signed URL is visible in shell history and process args — prefer --artifact-url-stdin)
      --artifact-url-stdin           read the --artifact-url value from stdin instead of the command line; keeps signed URLs out of shell history and process args; requires --device-platform
      --auto-terminate-minutes int   minutes until auto-termination; 0 disables; omitted uses the backend default (~5 days)
      --cluster string               target cluster name (use 'rde machine-type list --stack STACK_ID' to find candidates when the stack + machine type combo is ambiguous)
      --description string           session description
      --device-model string          device to boot: simctl device type ("iPhone 16") or emulator device profile ("pixel_7") — a screen profile, not that phone's firmware; default: the template's device model, else the platform default
      --device-os-version string     iOS only: an iOS version ("18.2") or simctl runtime id — anything else is rejected; default: the template's, else newest installed
      --device-platform string       boot a virtual device with the session: ios (simulator, macOS stack) or android (emulator, Linux stack); --stack/--machine-type may then be omitted; read 'rde device-guide' first
      --device-system-image string   Android only: the API-level knob — sdkmanager system image package ("system-images;android-34;google_apis;x86_64"); default: the template's, else the platform default
      --feature-flag stringArray     name of a feature flag to enable on the session (repeatable)
  -h, --help                         help for create
      --input stringArray            session input as [scope:]key=value (repeatable); unscoped applies to every scope that defines the key, scope:key (shared, on_create, on_claim, on_restore) to that scope only
  -l, --label stringArray            label to attach to the session as key=value (repeatable; at most 32; keys use letters, digits, and . _ / -, values additionally : and +; the bitrise.io/ key prefix is reserved)
      --machine-type string          machine type name for a template-less session, or to override the template's machine type (see 'rde machine-type list --stack STACK_ID')
      --map-saved-inputs             auto-fill template session inputs from the user's saved inputs (matched by key)
      --no-device                    create without the template's device (ignored when the template declares none)
      --owner string                 who owns the session: user (default; a personal session) or workspace (owned by the workspace itself, visible to every member; session inputs as plain values only). A Workspace API Token always creates workspace sessions
      --saved-input stringArray      session input as [scope:]key=savedInputID — uses a stored saved-input value (repeatable)
      --secret-input stringArray     session input as [scope:]key=value, stored as a secret at rest and hidden in 'session view' — on a direct create and on a claim alike (repeatable; the value is visible in shell history and process args — prefer --saved-input)
      --stack string                 stack ID for a template-less session, or to override the template's stack (see 'rde stack list')
      --template string              template ID or name to create the session from (omit to create a template-less session with --stack and --machine-type)
      --wait                         wait until the session leaves provisioning (running, failed, …) — with --device-platform until the device is ready or failed, on a lifecycle-hooks template until its hooks are done — before returning; exits 1 if the final status isn't running, the device failed or a hook failed
      --wait-timeout duration        max time to wait when --wait is set (uses Go duration syntax: 30s, 5m, 1h) (default 10m0s)
      --warm-pool string             warm pool ID or name to claim the session from: a pre-booted warm session is handed out instantly, or one is created from the pool's configuration when none is available; the pool fixes the configuration, so --template, --stack, --machine-type, --feature-flag, --cluster and the device flags are rejected with it; the input flags give the template's On claim and On restore inputs (see 'rde warm-pool list')
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

* [bitrise-cli rde session](bitrise-cli_rde_session.md)	 - Create, list, inspect, and manage RDE sessions

