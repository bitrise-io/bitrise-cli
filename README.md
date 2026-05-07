# Bitrise Platform CLI

A CLI tool to manage all Bitrise platform resources — CI, RM, RDE, and more — from your terminal.

## Commands

### `auth` — Manage the Bitrise access token

| Command | Description |
|---|---|
| `auth login` | Save a Bitrise access token |
| `auth logout` | Remove the saved access token |
| `auth status` | Show whether an access token is configured and where it came from |

### `app` — List, inspect, and manage apps (also: `project`)

| Command | Description |
|---|---|
| `app list` | List apps the authenticated user can access |
| `app view APP_SLUG` | Show details of a single app |

### `build` — Trigger, list, and inspect builds

| Command | Description |
|---|---|
| `build list` | List builds for an app |
| `build view BUILD_SLUG` | Show details of a single build |
| `build trigger` | Start a new build |
| `build trigger --wait` | Start a new build and stream logs until it finishes |
| `build log BUILD_SLUG` | Print the build log |
| `build watch BUILD_SLUG` | Stream logs for a running build until it finishes |

### `config` — Manage CLI configuration (defaults persisted to a YAML file)

| Command | Description |
|---|---|
| `config path` | Print the absolute path of the config file |
| `config list` | List the current config-file values |
| `config get KEY` | Print the value of a single config key |
| `config set KEY VALUE` | Set a config key and save the file |
| `config unset KEY` | Remove a config key and save the file |

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

**bash** — add to `~/.bashrc` or `~/.bash_profile`:
```bash
source <(bitrise-cli completion bash)
```

**zsh** — add to `~/.zshrc` (requires `compinit`, already enabled in Oh My Zsh):
```zsh
source <(bitrise-cli completion zsh)
```
Or install persistently:
```zsh
bitrise-cli completion zsh > "${fpath[1]}/_bitrise-cli"
```

**fish** — install once:
```fish
bitrise-cli completion fish > ~/.config/fish/completions/bitrise-cli.fish
```

**PowerShell** — add to your profile:
```powershell
bitrise-cli completion powershell | Out-String | Invoke-Expression
```
