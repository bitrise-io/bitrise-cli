# Bitrise Platform CLI

A CLI tool to manage all Bitrise platform resources — CI, RM, RDE, and more — from your terminal.

## Commands

The tables below are a hand-curated overview. The complete generated reference
— every command, flag, and example, straight from the command definitions —
lives in [docs/cli](docs/cli/bitrise-cli.md) and is kept in sync by CI
(`make docs` regenerates it).

### `auth` — Manage the Bitrise access token

| Command | Description |
|---|---|
| `auth login` | Prompt for a Personal Access Token and save it |
| `auth login --with-token` | Read a PAT from stdin (pipeline-friendly: `echo $TOKEN \| bitrise-cli auth login --with-token`) |
| `auth login --email <addr>` | Sign in with email + password and mint a fresh PAT |
| `auth logout` | Remove the saved access token |
| `auth status` | Show whether an access token is configured and where it came from |

### `user` — Create and manage your Bitrise account

| Command | Description |
|---|---|
| `user create --email <a> --username <u>` | Sign up for a new account; click the email link, then run `auth login --email <a>` |
| `user me` | Show the profile of the currently authenticated user |

### `app` — List, inspect, and manage apps (also: `project`)

| Command | Description |
|---|---|
| `app list` | List apps the authenticated user can access |
| `app view APP_SLUG` | Show details of a single app |
| `app create` | Register a new app (auto-detects repo URL/branch from cwd; saves the new slug as the default `app_slug`) |
| `app create --stack STACK_ID` | Set the build stack at creation time |
| `app create --bitrise-yml PATH` | Upload a bitrise.yml at creation time (also auto-uploads `./bitrise.yml` if present) |

### `build` — Trigger, list, and inspect builds

| Command | Description |
|---|---|
| `build list` | List builds for an app (newest first) |
| `build list --all` | Fetch all pages automatically instead of one page |
| `build list --branch B --workflow W --status S` | Filter by branch, workflow, or status (`success`, `failed`, `in-progress`, `aborted`, `aborted-with-success`) |
| `build view BUILD_SLUG` | Show details of a single build |
| `build trigger` | Start a new build (defaults to the `main` branch) |
| `build trigger --pipeline PIPELINE_ID` | Start a pipeline build (mutually exclusive with `--workflow`) |
| `build trigger --wait` | Start a build and block silently until it finishes; exits 1 on failure |
| `build trigger --watch` | Start a build and stream its logs until it finishes; exits 1 on failure |
| `build log BUILD_SLUG` | Print the build log (one-shot: current chunks for in-progress, full log for finished) |
| `build log BUILD_SLUG --wait` | Wait for the build to finish, then print the full log |
| `build watch BUILD_SLUG` | Stream logs for a running build until it finishes |
| `build abort BUILD_SLUG` | Abort a running or queued build |
| `build abort BUILD_SLUG --abort-with-success` | Abort and mark the build as successful |
| `build yml BUILD_SLUG` | Print the bitrise.yml a specific build ran with (shortcut for `yml get --build`) |

### `rde` — Manage Remote Dev Environments (sessions, templates, …)

Workspace resolution (highest to lowest precedence): `--workspace SLUG` → `BITRISE_WORKSPACE_ID` → `default_workspace_slug` config key. Saved inputs are user-scoped and do not require a workspace.

Anywhere a `SESSION_ID` or `TEMPLATE_ID` is accepted you can pass the resource's name instead — it's resolved to an ID for you. Names aren't unique, so if more than one resource matches the command errors and lists the candidate IDs to choose from.

| Command | Description |
|---|---|
| `rde session list` | List RDE sessions in the workspace |
| `rde session view SESSION_ID` | Show details of a single session (`--watch` to poll until Ctrl-C) |
| `rde session create NAME --template ID_OR_NAME` | Create a session from a template (`--input k=v`, `--secret-input k=v`, `--saved-input k=ID`, `--feature-flag F`, `--cluster C`, `--ai-prompt P`, `--auto-terminate-minutes N`, `--map-saved-inputs`, `--wait`) |
| `rde session update SESSION_ID` | Update a session's `--name`, `--description`, or `--auto-terminate-minutes` |
| `rde session restore SESSION_ID` | Restore a terminated session (re-provisions its VM from the persistent disk) |
| `rde session terminate SESSION_ID` | Terminate a running session (preserves it for later restart; `--wait` blocks until terminated/failed so `terminate --wait && delete` is reliable) |
| `rde session delete SESSION_ID` | Permanently delete a session |
| `rde session delete-terminated` | Delete every terminated session in the workspace (`--yes` to skip the prompt) |
| `rde session diff SESSION_ID` | Compare a session's template snapshot with the current template |
| `rde session notifications SESSION_ID` | List notifications from a session (`--since RFC3339`, `--before`, `--limit`, `--order asc\|desc`) |
| `rde session exec SESSION_ID -- CMD…` | Run a bash command on the session over SSH; returns `{exit_code, stdout, stderr}` in JSON mode |
| `rde session upload SESSION_ID LOCAL REMOTE_FOLDER` | tar.gz LOCAL and extract it on the session at REMOTE_FOLDER |
| `rde session download SESSION_ID REMOTE LOCAL` | Download REMOTE from the session into LOCAL (`--only-contents` for directories) |
| `rde session vnc SESSION_ID` | Print the VNC connection URL (human: single `vnc://user:pass@host:port` line; JSON: `{address, username, password, url}`) |
| `rde session open-vnc SESSION_ID` | Hand the VNC URL to the OS-default viewer (Screen Sharing on macOS, `xdg-open` on Linux, `cmd /c start` on Windows); password never hits stdout |
| `rde template list` | List templates in the workspace |
| `rde template view TEMPLATE_ID` | Show details of a single template |
| `rde template create --file FILE` | Create a template from a JSON spec file (or `--file -` for stdin) |
| `rde template update TEMPLATE_ID --file FILE` | Update a template from a JSON spec file (round-trip with `template view -o json`) |
| `rde template delete TEMPLATE_ID` | Delete a template (existing sessions keep working from their snapshot) |
| `rde saved-input list` | List saved inputs (user-scoped) |
| `rde saved-input view ID` | Show details of a single saved input |
| `rde saved-input create --key K --value V` | Create a saved input (`--secret`; `--value-stdin` reads the value from stdin, or it prompts when neither flag is given) |
| `rde saved-input update ID` | Update a saved input's `--value` (or `--value-stdin`) and/or `--secret` flag |
| `rde saved-input delete ID` | Delete a saved input |
| `rde image list` | List machine images available to the workspace |
| `rde machine-type list --image NAME` | List machine types compatible with the given image; the CLUSTER column is shown only when a machine type is offered by multiple clusters for that image (use that name as `--cluster` on `session create`) |

### `config` — Manage CLI configuration (defaults persisted to a YAML file)

| Command | Description |
|---|---|
| `config path` | Print the absolute path of the config file |
| `config list` | List the current config-file values |
| `config get KEY` | Print the value of a single config key |
| `config set KEY VALUE` | Set a config key and save the file |
| `config unset KEY` | Remove a config key and save the file |

Recognized keys: `output`, `app_slug`, `default_workspace_slug`, `api_base_url`, `rde_api_base_url`, `web_base_url`, `theme`.

### `yml` — Get, update, or validate the bitrise.yml stored on Bitrise

| Command | Description |
|---|---|
| `yml get` | Print the stored bitrise.yml for an app (bare `yml` also works) |
| `yml get --build BUILD_SLUG` | Print the bitrise.yml a specific build ran with |
| `yml update` | Upload a new bitrise.yml (from `--file` or stdin) |
| `yml validate` | Validate a bitrise.yml against the schema; exits 1 if invalid |
| `yml validate --app APP_SLUG` | Validate with app-specific settings (available stacks, machine types) |

### `stack` — List available stacks

| Command | Description |
|---|---|
| `stack list` | List available stacks with OS and status |
| `stack list --workspace SLUG` | List stacks available for a specific workspace |

### `step` — Search steps and inspect their inputs

| Command | Description |
|---|---|
| `step search QUERY` | Find steps by name, description, or tags |
| `step search QUERY --category CAT --maintainer M` | Filter results by category and maintainer |
| `step inputs STEP_REF` | List inputs for a step version (e.g. `git-clone@8.3.1`) |

### `version` — Print version, commit, and build info

| Command | Description |
|---|---|
| `version` | Print version, commit, and build info |

### `completion` — Generate shell completion scripts

| Command | Description |
|---|---|
| `completion bash` | Generate bash completion script |
| `completion zsh` | Generate zsh completion script |
| `completion fish` | Generate fish completion script |
| `completion powershell` | Generate PowerShell completion script |

## Shell completion

Tab-completion is available for all commands, subcommands, flags, and known flag values.

| Shell | Load now | Make permanent |
|---|---|---|
| bash | `source <(bitrise-cli completion bash)` | add to `~/.bashrc` |
| zsh | `source <(bitrise-cli completion zsh)` | add to `~/.zshrc` |
| fish | `bitrise-cli completion fish \| source` | `bitrise-cli completion fish > ~/.config/fish/completions/bitrise-cli.fish` |
| PowerShell | `bitrise-cli completion powershell \| Out-String \| Invoke-Expression` | add to `$PROFILE` |
