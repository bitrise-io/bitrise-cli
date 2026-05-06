# CLAUDE.md

## REQUIRED READING — every session, before any change

**All sessions must follow the Bitrise CLI Patterns Guide:**

https://bitrise.atlassian.net/wiki/x/EoBwKQE

That doc is the source of truth for CLI conventions: command structure,
noun/verb naming, output formats, flag conventions, config precedence, help
format, stdout/stderr discipline, exit codes. Re-read it before changing any
user-facing surface (flags, command names, error messages, help text) or
adding new commands. If a proposed change conflicts with the guide, raise
the conflict — don't go around it.

Project-scope (which P-priorities are in flight right now):
https://bitrise.atlassian.net/wiki/spaces/~7120208440da8e0559401d8ca71c0dd078a47f/pages/4990697487/BR+Proj+Scope

## What this is

`bitrise-cli` is a new CLI for Bitrise platform resources (builds, apps,
workflows). It is currently in **stub mode** — `cmd/` handlers go through
service stubs in `internal/build` and `internal/app` that return canned
data. The HTTP client in `bitriseapi/` exists (`Me()` works) but is not
wired into any command yet. Don't add real API calls without confirming
scope first.

The canonical binary name is `bitrise-cli`. `br` is documented as an
optional shell alias / symlink, NOT shipped as the binary name. The
patterns guide flags a real collision with [broot]'s `br` shell function;
do not rename the binary to `br` without a team decision.

## Layering — keep it intact

```
cmd/                 cobra presentation only: flag parsing, output formatting,
                     calling into services. NO business logic, NO HTTP.
internal/build       service stubs for build operations
internal/app         service stubs for app + workflow operations
internal/auth        Auth file (auth.yaml): the access token only
internal/config      Config + Path/Load/Save, LoadDir, Resolve, ctx helpers
internal/output      Format + generic Render; Human and JSON formats
bitriseapi/          HTTP client (existing). Not yet called from cmd handlers.
```

cmd handlers do exactly: parse flags → call service method → render result.
When wiring real API calls, the cmd layer doesn't change — only the
internal services gain a `*bitriseapi.Client`.

## Locked-in conventions (per the patterns guide)

- **Output format flag**: `--output human|json`, `-o`. `human` is the
  default. JSON output is a **stable contract** — additive changes only;
  no breaking renames without a major version bump.
- **stdout vs stderr**: stdout carries the answer (data, JSON, table rows).
  stderr carries diagnostics, confirmations, progress. JSON mode never
  mixes diagnostics into stdout. Errors via cobra's RunE go to stderr.
- **Config precedence** (highest to lowest):
  1. CLI flag (`--output` folded in by `persistentPreRun`; per-command
     flags like `--app` are layered in the command handler itself)
  2. Environment variables (`BITRISE_TOKEN`, `BITRISE_APP_SLUG`,
     `BITRISE_OUTPUT`, `BITRISE_API_BASE_URL`)
  3. Per-directory file: `.bitrise-cli.yml` in CWD or any ancestor
  4. Global file: `$XDG_CONFIG_HOME/bitrise/config.yaml`
     (falls back to `~/.config/bitrise/config.yaml`)
  5. Auth file (token + type only): `$XDG_CONFIG_HOME/bitrise/auth.yaml`
  6. Built-in defaults
- **Token storage**: `bitrise-cli auth login` writes the token to
  `auth.yaml`. PAT and WAT tokens are stored identically (same wire format —
  no type field). Resolve order for tokens: env > auth.yaml.
- **File perms**: both `config.yaml` and `auth.yaml` are 0600; parent dir 0700.
- **Bitrise verbs**: `build trigger` (not create); `build abort` (not
  cancel) when added; `build rerun` for re-runs; `view` is the detail verb.
- **Singular nouns**: `app`, `build`, `workflow` — never plural.
- **`app` ↔ `project` aliases**: both the command (`bitrise-cli project ...`)
  and the flag (`--project`) accept either form. `app` is canonical.
- **Stdin via `-`**: `bitrise-cli config set token -` reads from stdin so
  secrets stay out of shell history. Apply this pattern to any new
  secret-accepting command.
- **`-q`/`--quiet`** suppresses non-error stderr ("Saved output", etc.).
  Errors and primary stdout output ignore it.

## Deferred — don't add without a ticket

These are listed in the patterns guide as standard features but are
intentionally out of scope right now. Don't reopen the discussion as part
of an unrelated change:

- `--web` flag (open in dashboard) — needs real URLs first
- `bitrise-cli api` raw HTTP wrapper
- OAuth login flow (current `auth login` is token-paste only, no OAuth)
- `--json fields` projection + `--jq` expression
- Color support + `NO_COLOR`/`FORCE_COLOR`
- `--watch` / `--wait` (build streaming)
- `--dry-run` for mutating commands
- Workspace concept (`workspace use`, `--workspace`)
- Confirmation prompts on destructive ops (no destructive ops exist yet)
- OS-keychain token storage (currently in `auth.yaml` 0600)
- PAT vs WAT type tagging — they have identical wire format, so we store
  them as one opaque token. Add a type field back if/when cross-workspace
  operations gain WAT-aware warnings.
- `bitrise.yml`-based context auto-detection
- Telemetry, update checks, plugin system, `init` wizard
- Per-directory config writing via `bitrise-cli config set` (currently
  set/unset only modify the global file; per-dir is hand-edited)

## Build, vet, run

- `go build -o ./bitrise-cli .` — binary lands at the repo root, gitignored
- `go mod tidy && git diff --exit-code go.mod go.sum` — dependency hygiene check
- `gofmt -l .` — formatting check (must produce no output)
- `go vet ./...` — static analysis
- Lint: binary is cached at `./bin/golangci-lint-<version>` and downloaded on first use via the official install script (see the `Lint` step in `bitrise.yml`). Run `./bin/golangci-lint-v2.12.2 run ./...` locally once it exists.
- `go test -race -count=1 -timeout=5m ./...` — tests
- Run all of the above via `bitrise run test`
- When adding tests, put them in the same package as the file under test
- `go.mod` is at module path `github.com/bitrise-io/bitrise-cli`

## Versioning hooks

`cmd.version` and `cmd.commit` are package-level `var`s so CI can inject
real values via `-ldflags`:

```
go build -ldflags "-s -w \
                  -X github.com/bitrise-io/bitrise-cli/cmd.version=X.Y.Z \
                  -X github.com/bitrise-io/bitrise-cli/cmd.commit=$GIT_SHA"
```

When ldflags aren't set, `runtime/debug.ReadBuildInfo()` fills in
`vcs.revision` and `vcs.time` so `bitrise-cli version` still has commit info.

## Known nits

- Cobra auto-binds `-v` to `--version`. The patterns guide reserves `-v` for
  `--verbose`. Reclaim it when adding `--verbose`.
- A malformed config file (global or per-dir) makes every command fail,
  including `bitrise-cli config list/set/unset`. Recovery is hand-editing
  or deleting the file. A `config reset` escape hatch is a reasonable
  follow-up.

## README command list

`README.md` contains a full command reference table. **Keep it in sync**:
whenever a command is added, renamed, or removed, update the corresponding
row (or section) in the README as part of the same change.

## When in doubt

Open the patterns guide (https://bitrise.atlassian.net/wiki/x/EoBwKQE) and
follow what it says. If the guide doesn't cover the case, mirror what `gh`
does — that's the closest-spirit reference CLI for our use case.
