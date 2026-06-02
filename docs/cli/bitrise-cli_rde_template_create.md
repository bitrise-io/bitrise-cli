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
  image          machine image name (see 'rde image list')
  machine_type   machine type name (see 'rde machine-type list')

Optional fields:
  description         free-text description
  working_directory   default working directory for sessions
  startup_script      bash script run on every session start
  warmup_script       bash script baked into the image during pre-warming
  session_inputs      array of {key, description, required, default_value,
                      expose_as_env_var} — values callers supply at create time
  template_variables  array of {key, value, is_secret, expose_as_env_var} —
                      baked-in values available to startup/warmup scripts
  feature_flags       array of {name, description}
  workspace_links     array of {label, folder_path, feature_flag_name} — IDE
                      folder shortcuts

Example spec exercising every field (a macOS iOS-app dev environment —
adjust to taste):

  {
    "name": "example-ios-app",
    "description": "Example macOS dev environment for an iOS app.",
    "image": "osx-sequoia-26",
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
    ]
  }

```
bitrise-cli rde template create [flags]
```

### Examples

```
  bitrise-cli rde template create --file template.json
  cat template.json | bitrise-cli rde template create --file -
```

### Options

```
  -f, --file string   path to a JSON spec file (use '-' for stdin)
  -h, --help          help for create
```

### Options inherited from parent commands

```
      --no-color           disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string      output format: human|json (default "human")
  -q, --quiet              suppress non-error diagnostic messages
      --theme string       color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
      --workspace string   workspace slug (or set BITRISE_WORKSPACE_ID; defaults to default_workspace_slug)
```

### SEE ALSO

* [bitrise-cli rde template](bitrise-cli_rde_template.md)	 - List and inspect RDE templates

