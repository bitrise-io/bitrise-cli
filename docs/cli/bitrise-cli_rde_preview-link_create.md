## bitrise-cli rde preview-link create

Mint a shareable device preview link for an app build

### Synopsis

Mint a shareable link that opens an app build on a live iOS simulator or
Android emulator in the recipient's browser — no Bitrise login, no local
tooling, nothing for them to install. The usual shape is a CI step that builds
a simulator/emulator app and posts the link on the pull request.

The app build (--artifact-url) must be reachable by a plain anonymous GET — a
presigned URL is fine, and it is never shown to viewers. It must be a
SIMULATOR/EMULATOR build, not a device build: iOS wants a zipped .app carrying
an arm64 simulator slice, Android an .apk that runs on x86_64. A device build
installs on neither, and the viewer only finds out after a device has booted.

The link is a bearer credential. Anyone holding the URL can open a device on
this workspace's bill, and there is no way to revoke one — a short --ttl is the
control. Post it on a pull request or a team channel, never a public one.

Nothing is stored when the link is minted: this output is the only record that
will exist, so capture the URL. A link nobody opens costs nothing, and every
open gets its own device, so one link serves several reviewers at once.

Limits: --ttl defaults to 24 hours and is capped at 72. At most 5 devices alive
per link and 20 per workspace; opens past the cap are refused. Each device
auto-terminates after its idle window (--auto-terminate-minutes, at least 10,
default 60).

In CI, authenticate with a Workspace API Token rather than a personal one:
minting preview links is one of the few RDE operations a workspace token may
perform, and the sessions it opens belong to the workspace instead of a person.
Pass it as BITRISE_TOKEN, which is used verbatim, and name the workspace with
--workspace or BITRISE_WORKSPACE_ID — a workspace token belongs to one
workspace and cannot look up which workspaces an account has, so leaving it to
be auto-detected fails.

Warm pools: pass --warm-pool to serve the link's opens from a workspace-owned
warm pool ('rde warm-pool list'). Each open claims one of the pool's
pre-booted device sessions when one is available — the click-to-app time is
the app download, not a VM boot — and creates a session from the pool's
configuration otherwise. The pool fixes the device and the machine, so omit
--device-platform and the other --device-* flags, --stack and --machine-type
with it; the pool's configuration must boot a device. A pool deleted after
minting degrades later opens to the ordinary cold path.


```
bitrise-cli rde preview-link create [flags]
```

### Examples

```
  bitrise-cli rde preview-link create --device-platform android --artifact-url https://…/app.apk
  # Serve opens from a warm pool's pre-booted devices (the pool fixes the device).
  bitrise-cli rde preview-link create --warm-pool WARM_POOL_ID --artifact-url https://…/app.apk
  bitrise-cli rde preview-link create --device-platform ios --artifact-url https://…/App.zip --device-model "iPhone 16"
  bitrise-cli rde preview-link create --device-platform ios --artifact-url-stdin < artifact-url.txt
  bitrise-cli rde preview-link create --device-platform android --artifact-url https://…/app.apk --ttl 4h
  bitrise-cli rde preview-link create --device-platform android --artifact-url https://…/app.apk --output json | jq -r .url

  # From CI, with a Workspace API Token (--workspace is required with one):
  BITRISE_TOKEN=bitwat_… bitrise-cli rde preview-link create --workspace WORKSPACE_ID \
    --device-platform ios --artifact-url https://…/App.zip
```

### Options

```
      --artifact-build-number string   build number shown on the viewer page
      --artifact-commit string         commit SHA shown on the viewer page
      --artifact-name string           app display name shown on the viewer page
      --artifact-url string            app build to install on every open: absolute http(s) URL of a zipped simulator .app (iOS) or an .apk (Android), fetchable by anonymous GET (a signed URL is visible in shell history and process args — prefer --artifact-url-stdin)
      --artifact-url-stdin             read the --artifact-url value from stdin instead of the command line; keeps signed URLs out of shell history and process args
      --auto-terminate-minutes int     minutes a device stays alive after its last viewer disconnects; 0 uses the default of 60; minimum 10, maximum 480
      --device-model string            device to boot: simctl device type ("iPhone 16") or emulator device profile ("pixel_7") — a screen profile, not that phone's firmware; default: the platform default
      --device-os-version string       iOS only: an iOS version ("18.2") or simctl runtime id — anything else is rejected; default: newest installed
      --device-platform string         device each open boots: ios (simulator) or android (emulator) (required)
      --device-system-image string     Android only: the API-level knob — sdkmanager system image package ("system-images;android-34;google_apis;x86_64"); default: the platform default
  -h, --help                           help for create
      --machine-type string            machine type the link's devices run on; omit for the platform default (see 'rde machine-type list --stack STACK_ID')
      --stack string                   stack the link's devices run on; omit for the platform default (see 'rde stack list')
      --ttl duration                   how long the link stays openable (Go duration syntax: 4h, 30m); 0 uses the default of 24h, maximum 72h
      --warm-pool string               workspace-owned warm pool (ID or name) whose pre-booted device sessions serve the link's opens; the pool fixes the device and the machine, so omit the --device-* flags, --stack and --machine-type (see 'rde warm-pool list')
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

* [bitrise-cli rde preview-link](bitrise-cli_rde_preview-link.md)	 - Create shareable device preview links for app builds

