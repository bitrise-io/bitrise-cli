## bitrise-cli rde template create

Create a new RDE template from a JSON spec file

### Synopsis

Create a new RDE template from a JSON spec file. See the annotated
example at the bottom for the JSON shape — or, once you have a template
you like, round-trip from it (audit fields like id/created_at are ignored):

  bitrise-cli rde template view OTHER_TEMPLATE_ID -o json > template.json
  # edit template.json
  bitrise-cli rde template create --file template.json

Pass --file - to read the JSON from stdin.

Required fields:
  name           template name
  stack_id       stack ID (see 'rde stack list')
  machine_type   machine type name (see 'rde machine-type list')

Optional fields:
  description         free-text description
  working_directory   default working directory for sessions
  startup_script      bash script run on every session start (legacy pair)
  warmup_script       bash script run once during pre-warming (legacy pair)
  hooks               {on_create, on_claim, on_restore} — lifecycle hooks, the
                      alternative to the legacy pair (see below)
  session_inputs      array of {key, description, required, default_value,
                      expose_as_env_var, hook} — values callers supply when
                      creating (or claiming) a session; hook is the input's
                      scope (see "Scopes" below)
  template_variables  array of {key, value, is_secret, protected,
                      expose_as_env_var, hook} — baked-in values the scripts
                      receive; hook is the variable's scope (see below)
  feature_flags       array of {name, description}
  workspace_links     array of {label, folder_path, feature_flag_name} — IDE
                      folder shortcuts
  device_spec         {platform, device_model, os_version, system_image} — a
                      virtual device sessions created from the template boot
                      unless overridden (see below)

Declared device: pass --device-platform ios or android (plus optionally
--device-model, --device-os-version, --device-system-image — the same flags
'rde session create' takes) to declare a virtual device on the template.
Sessions created from the template then boot that device as declared;
'rde session create' can tweak it per field with --device-model etc. (no
--device-platform), replace it (with --device-platform) or skip it with
--no-device. The template's stack and machine type
must fit the platform (iOS: a macOS stack; Android: a dockerless Android
Linux stack). The flags take precedence over a device_spec in the file.
Read 'bitrise-cli rde device-guide' before declaring one.

Lifecycle hooks: instead of startup_script/warmup_script a template can
carry hooks — bash scripts the platform runs on the booted machine as the
session user, with the values of the hook's scope exported. Every hook is
optional. on_create runs once when the session is created (on a warm pool:
when the pool warms the machine). on_claim runs on every claim, when the
session is assigned to its owner. on_restore runs every time a stopped
session is restored — background processes do not survive a stop, so
anything that must run again belongs here too. A hook runs when it is in
the hooks object, even with an empty script ("" finishes at once, exit 0);
leave a hook out to not have it: with no on_create the session (or a warm
pool machine) is set up as soon as it boots, and a template with no hooks
at all is valid. A hook is done when its script exits (0 = done, non-zero =
failed): start long-running work, like a dev server, in the background so
the script can exit — without the values it must not keep ('env -u SECRET
cmd &'). An On create that has not exited after 20 minutes is ended by RDE
(exit code 124). While On create is pending or running the session cannot
be reached: exec, ssh, upload, download and the terminal are refused
("Session is setting up: On create is still running. Access opens when it
finishes.") — 'session create --wait' returns once the setup is done. hooks
cannot be combined with startup_script/warmup_script, and saving them
requires lifecycle hooks to be available on the workspace. 'session view'
shows the setup phase of a session created from such a template, and
'session logs --stage on-create' its On create output.

Scopes: every variable and session input belongs to one scope, its hook:
"shared" (the default — the key may be left out) reaches every hook;
"on_create", "on_claim" or "on_restore" reach that hook only and need a
hooks template. A hook receives the shared values plus its own, nothing
else (On restore does not see the claim values: give it its own inputs or
make them shared); its own value wins over a shared one of the same key.
Keys are unique within a scope, and the same key may exist on several
scopes — one value, or one saved input, then fills all of them. Who
supplies the inputs: a direct 'session create' asks for every required
input of every scope; a warm pool is created with the shared and on_create
inputs; whoever claims supplies the on_claim and on_restore ones. Legacy
templates, and specs without a hook key, keep every value shared — exactly
the delivery they had.

Variables: is_secret encrypts the value at rest and hides it in output
(members with access can still read it with include_secrets). protected
goes further — secret and write-only: the API never returns the value,
'template view' shows the key only, and an empty value on update keeps the
stored one (so the round-trip below keeps it). Use protected for secrets
every warm pool of the template reuses, such as a registry credential. A
protected value is never written to the machine, so expose_as_env_var is
refused with it.

Also in terminal (expose_as_env_var, on variables and inputs of every
scope): hooks always receive this value. Also in terminal additionally
writes it to the session's shell profile when this step runs, so it is
available in SSH and terminal sessions afterwards. Later steps override
earlier ones for the same name. Exposed Shared and On create values stay on
a pool machine for whoever claims it. A hook with no script but with
exposed values still gets a run, so the values are applied.

Example spec exercising every field (a macOS iOS-app dev environment —
adjust to taste):

  {
    "name": "example-ios-app",
    "description": "Example macOS dev environment for an iOS app.",
    "stack_id": "osx-xcode-16.0.x-edge",
    "machine_type": "g2.mac.m2pro.6c-14g",
    "working_directory": "/Users/vagrant/git",
    "warmup_script": "set -euo pipefail\ncd ~\ngit clone \"https://${GITHUB_PAT}@github.com/example-org/example-app.git\" git\ncd git && bundle install && pod install --project-directory=ios\n",
    "startup_script": "set -euo pipefail\ncd /Users/vagrant/git\ngit pull --ff-only || true\nsudo xcode-select -s \"/Applications/Xcode-${XCODE_VERSION}.app\"\n",
    "session_inputs": [
      {
        "key": "GITHUB_PAT",
        "description": "GitHub PAT with read access to example-org/example-app",
        "required": true,
        "expose_as_env_var": true
      },
      {
        "key": "XCODE_VERSION",
        "description": "Xcode version to select via xcode-select",
        "default_value": "26.3",
        "expose_as_env_var": true
      }
    ],
    "template_variables": [
      {"key": "APP_SCHEME", "value": "ExampleApp", "expose_as_env_var": true},
      {"key": "FASTLANE_API_KEY", "is_secret": true, "expose_as_env_var": true}
    ],
    "feature_flags": [
      {"name": "enable_beta_simulator", "description": "Boot the iOS beta simulator on session start"}
    ],
    "workspace_links": [
      {"label": "Open app in Xcode", "folder_path": "/Users/vagrant/git/ios"},
      {"label": "Open scripts (beta only)", "folder_path": "/Users/vagrant/git/scripts", "feature_flag_name": "enable_beta_simulator"}
    ],
    "device_spec": {"platform": "ios", "device_model": "iPhone 16", "os_version": "18.2"}
  }

```
bitrise-cli rde template create [flags]
```

### Examples

```
  bitrise-cli rde template create --file template.json
  cat template.json | bitrise-cli rde template create --file -
  # Declare an iOS simulator every session from this template boots by default.
  bitrise-cli rde template create --file template.json --device-platform ios --device-model "iPhone 16"
  # Lifecycle hooks instead of the legacy scripts (On create clones and installs; On restore refreshes).
  # GH_TOKEN is asked from whoever claims (on_claim) and again for On restore; REGISTRY_TOKEN is a protected On create variable.
  echo '{"name":"hooks-dev","stack_id":"linux-ubuntu-24.04","machine_type":"standard","hooks":{"on_create":"git clone https://github.com/example-org/app.git ~/app && cd ~/app && npm ci","on_restore":"cd ~/app && git pull --ff-only"},"session_inputs":[{"key":"GH_TOKEN","required":true,"hook":"on_claim"},{"key":"GH_TOKEN","required":true,"hook":"on_restore"}],"template_variables":[{"key":"REGISTRY_TOKEN","value":"...","hook":"on_create","protected":true},{"key":"APP_ENV","value":"dev","expose_as_env_var":true}]}' | bitrise-cli rde template create --file -
```

### Options

```
      --device-model string          device to boot: simctl device type ("iPhone 16") or emulator device profile ("pixel_7"); default: platform default; requires --device-platform. "iPhone Duo" needs an Xcode 27.1+ stack (the session or template stack is validated)
      --device-os-version string     iOS only: an iOS version ("18.2") or simctl runtime id — anything else is rejected; default: newest installed; requires --device-platform
      --device-platform string       declare a virtual device sessions created from the template boot unless overridden: ios (simulator, macOS stack) or android (emulator, Linux stack); read 'rde device-guide' first
      --device-system-image string   Android only: sdkmanager system image package ("system-images;android-34;google_apis;x86_64"); default: platform default; requires --device-platform
  -f, --file string                  path to a JSON spec file (use '-' for stdin)
  -h, --help                         help for create
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

* [bitrise-cli rde template](bitrise-cli_rde_template.md)	 - List and inspect RDE templates

