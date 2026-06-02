## bitrise-cli app create

Register a new app on Bitrise

### Synopsis

Register a new app (project) on Bitrise.

Auto-detection from the current git repo:
  --repo-url     git remote get-url origin
  --branch       git symbolic-ref --short HEAD (else "master")
  --title        last path segment of the repo URL (".git" stripped)
  --provider     derived from the repo URL host (github.com → github, etc.)

Workspace:
  --workspace is required if you belong to multiple workspaces.
  Otherwise it falls back to:
    1. default_workspace_slug from config
    2. auto-detect when your account has exactly one workspace

bitrise.yml handling:
  --bitrise-yml PATH   upload that file as the app's config
  (no flag, ./bitrise.yml exists)   upload it
  (no flag, no file)   skip — server preset for --project-type takes effect

The new app's slug is saved as the global default app_slug, so subsequent
commands (build trigger, build list, ...) target it without --app.

```
bitrise-cli app create [flags]
```

### Examples

```
  bitrise-cli app create
  bitrise-cli app create --repo-url https://github.com/me/proj --workspace acme
  bitrise-cli app create --bitrise-yml ./ci/bitrise.yml --stack osx-xcode-16.0.x
  bitrise-cli app create --output json
```

### Options

```
      --bitrise-yml string    path to bitrise.yml to upload (default: ./bitrise.yml if present, else skip)
      --branch string         default branch (default: 'git symbolic-ref --short HEAD', else 'master')
  -h, --help                  help for create
      --project-type string   project type for server-side preset (default "other")
      --provider string       git provider: auto, github, gitlab, bitbucket, custom (default "auto")
      --public                create as a public app
      --repo-url string       git repo URL (default: 'git remote get-url origin' in cwd)
      --stack string          build stack ID (default "linux-docker-android-22.04")
      --title string          app title (default: last path segment of repo URL)
      --workspace string      workspace slug to own the app
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli app](bitrise-cli_app.md)	 - List, inspect, and manage apps (also: project)

