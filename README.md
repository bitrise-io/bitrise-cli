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
| `build log BUILD_SLUG` | Print the build log |

### `config` — Manage CLI configuration

| Command | Description |
|---|---|
| `config path` | Print the absolute path of the config file |
| `config list` | List the current config-file values |
| `config get KEY` | Print the value of a single config key |
| `config set KEY VALUE` | Set a config key and save the file |
| `config unset KEY` | Remove a config key and save the file |

### `version` — Print version, commit, and build info
