# bitrise-cli Output Scheme

Reference for how `bitrise-cli` formats and delivers output to the user.
Covers the data channel (stdout), the diagnostic channel (stderr), colors,
symbols, table layout, and JSON contracts.

---

## 1. Two-channel model

Every output falls into one of two channels:

| Channel | Stream | Content |
|---------|--------|---------|
| **Data** | stdout | The answer — tables, key/value views, JSON |
| **Diagnostics** | stderr | Confirmations, warnings, hints |

JSON mode never injects diagnostics into stdout. Redirecting or piping stdout
always yields clean, parse-safe data.

---

## 2. Output formats

Controlled by `--output human|json` (`-o`), the env var `BITRISE_OUTPUT`, or
the `output` key in config files.

| Format | Description |
|--------|-------------|
| `human` | Default. Styled tables and key/value text. ANSI codes on TTY. |
| `json` | Indented JSON (2-space). Stable contract — additive-only changes. |

`internal/output.Render[T]` dispatches between the two:

```go
output.Render(cmd.OutOrStdout(), format, data, renderHumanFn)
```

In JSON mode `renderHumanFn` is never called; `data` is marshalled directly.

---

## 3. Unicode symbols

Symbols appear only in human-format output. JSON output never contains them —
the JSON path never calls the human renderer.

| Symbol | Unicode | Color | Meaning |
|--------|---------|-------|---------|
| `✓` | U+2713 | green | Success / confirmation |
| `✗` | U+2717 | red | Failure / error state |
| `→` | U+2192 | plain | Relationship indicator (e.g. PR → target branch) |

**Current uses in the codebase:**

```
✓  auth status: access token configured  (renderAuthStatusHuman → stdout)
✗  auth status: no token configured      (renderAuthStatusHuman → stdout)
✓  build trigger: build triggered        (renderTriggerHero → stdout)
→  build view: Pull Request: #42 → main  (renderBuildText → stdout)
```

No symbol is used for neutral info or hint text — plain text only.

---

## 4. Color palette

Defined in `internal/output/style/style.go`. All values are 256-color ANSI
indices; lipgloss automatically falls back to the nearest ANSI-16 color on
legacy terminals.

| Constant | ANSI 256 | Purpose |
|----------|----------|---------|
| `colorGrey` | `245` | Dim text, table headers, IDs |
| `colorGreen` | `42` | Success |
| `colorRed` | `196` | Failure |
| `colorBlue` | `33` | In-progress / running |
| `colorAmber` | `214` | Aborted |
| `colorYellow` | `220` | Warnings |

### Semantic styles

`style.New(w io.Writer)` returns a `Styles` bundle with these public fields:

| Style name | Appearance | Used for |
|-----------|------------|----------|
| `Header` | bold grey | Table column headers |
| `Dim` | grey | Secondary text, file paths, pagination hints, disabled rows |
| `Bold` | bold | App/binary name, primary emphasis |
| `Label` | bold | Key side of key/value pairs |
| `Slug` | grey | Technical identifiers (app ID, commit hash, build ID) |
| `URL` | underlined | Hyperlinks (PR URL, build URL) |
| `Success` | green | `✓` symbol and confirmation lines |
| `Failure` | red | `✗` symbol and failure indicators |
| `Warn` | yellow | Warning lines |

### Build-status styles

`Styles.BuildStatus(status string)` maps an API status string to a style:

| API status | Color |
|------------|-------|
| `success` | green |
| `failed` | red |
| `aborted` | amber |
| `aborted-with-success` | amber |
| `in-progress` | blue |
| *(unknown / future)* | grey |

### Color detection and control

Colors are **per-writer**. `style.New(w)` binds to `w`'s terminal capabilities:

- Non-TTY writers (`*bytes.Buffer`, pipes) produce ANSI-free output automatically.
- `NO_COLOR` and `CLICOLOR` / `CLICOLOR_FORCE` env vars are honoured by the
  underlying `charmbracelet/lipgloss` + `muesli/termenv` stack.
- `--no-color` flag calls `style.Configure(true)` once in `persistentPreRun`,
  forcing every subsequent `style.New()` call to the Ascii (no-color) profile.
- `style.HasColor()` returns `false` when the Ascii profile is active — useful
  for conditional icon rendering in tests.

---

## 5. Table scheme

Rendered by `style.Table` in `internal/output/style/style.go`.

```go
style.Table(w, headers, rows, s.Header, cellStyler)
```

### Layout rules

- **No borders, no separators.** Columns are separated by two-space gutters.
- **ALL-CAPS column headers** styled with `s.Header` (bold grey).
- **Dynamic column widths.** Each column is as wide as its widest cell or
  header, measured with `lipgloss.Width()` so ANSI codes don't break alignment.
- **Per-cell styling** via a `CellStyler` callback:
  `func(row, col int, content string) string`
  `row == -1` is the header row. The callback must not change the visible width.

### Standard column conventions

| Column type | Style | Notes |
|-------------|-------|-------|
| Status | `s.BuildStatus(status)` | Color varies by API value |
| ID / hash | `s.Slug` (grey) | Always dimmed |
| Disabled row | `s.Dim` (full row) | Applied when an app is disabled |
| Everything else | plain | No additional styling |

### Example — build list

```
NUMBER  STATUS      BRANCH   WORKFLOW  TRIGGERED         ID
42      success     main     primary   2024-01-15 14:30  stub-build-aaa
41      in-progress feature  primary   2024-01-15 14:25  stub-build-bbb
```

- Headers: bold grey.
- `success` → green; `in-progress` → blue.
- Slug column → grey.

### Example — app list

```
TITLE    PROVIDER  PROJECT_TYPE  WORKSPACE        DISABLED  ID
My App   github    ios           workspace/alice            my-app-id
Old App  github    android       workspace/alice  yes        old-app-id
```

- Disabled rows: every cell is grey.
- Slug column: grey on enabled rows.

### Empty state

When there are no results, a single plain-text line replaces the table:

```
No builds found.
No apps found.
```

### Pagination hint

When a next page is available, a dimmed hint line follows the table after one
blank line:

```
More results available — pass --cursor eyJ...
```

---

## 6. Key/value detail view

Used by `build view`, `app view`, `auth status`, `version`, and similar
detail commands.

### Layout

Labels are bold, padded to a fixed width with `fmt.Sprintf("%-16s", label)`,
aligning values into a column. No separator character between label and value.

```
Build:          #42 (stub-build-aaa)
App:            my-app-id
Status:         in-progress
Workflow:       primary
Branch:         main
Pull Request:   #7 → main
PR URL:         https://github.com/owner/repo/pull/7
Commit:         abc1234def5678
Triggered:      2024-01-15 14:30:00 UTC
```

### Per-field styling

| Field type | Style applied |
|------------|---------------|
| Label | `s.Label` (bold) |
| Status value | `s.BuildStatus(status)` |
| ID / hash / build ID | `s.Slug` (grey) |
| File path | `s.Dim` (grey) |
| URL | `s.URL` (underlined) |
| PR arrow | `→` plain text |
| Numbers / plain text | unstyled |

### Conditional fields

Fields are omitted entirely when empty or zero — no `N/A` placeholders.
Examples: `StatusText`, `AbortReason`, `Tag`, `PullRequestID`, `CommitHash`,
`FinishedAt`, `StackIdentifier`, `MachineTypeID`.

---

## 7. Diagnostic messages (stderr)

Confirmations and warnings are written directly to `cmd.ErrOrStderr()`.

### Message types

| Type | Symbol | Color | Suppressed by `--quiet`? |
|------|--------|-------|--------------------------|
| Success confirmation | *(none)* | plain | yes — guarded by `if !quiet` |
| Warning | *(none currently)* | plain | no |
| Error | *(none)* | — | no — returned as Go error, printed by Cobra |

### Text conventions

| Message type | Format | Example |
|-------------|--------|---------|
| Success confirmation | past tense, no period | `Saved access token` |
| Warning | sentence case | `Token not set; commands will use the public API` |
| Hint | plain text, 2-space indent | `  Run 'bitrise-cli auth login'` |
| Error (Go error) | lowercase `verb: detail` | `load config: file not found` |

### Current call sites

```go
// cmd/auth.go — after writing auth.yaml
if !quiet {
    fmt.Fprintln(cmd.ErrOrStderr(), "Saved access token")
}

// cmd/config/cmd.go — after saving a config key
if !cmdutil.IsQuiet(cmd) {
    fmt.Fprintf(cmd.ErrOrStderr(), "Saved %s\n", key)
}
```

Quiet-mode checks are per-call-site. The package-level `quiet` var is in
`cmd/` and subpackages use `cmdutil.IsQuiet(cmd)`.

---

## 8. ErrWriter pattern

Multi-line human renderers use `cmdutil.NewErrWriter` instead of per-write
error checks. Defined in `cmd/cmdutil/cmdutil.go`.

```go
func renderExample(w io.Writer, data MyData) error {
    s := style.New(w)
    ew := cmdutil.NewErrWriter(w)

    ew.F("%s %s\n", s.Bold.Render("bitrise-cli"), data.Version)
    ew.F("%s%s\n", s.Label.Render(fmt.Sprintf("%-12s", "commit:")), s.Slug.Render(data.Commit))
    ew.Ln()
    return ew.Err  // first write error, or nil
}
```

`ErrWriter.F` and `ErrWriter.Ln` are no-ops after the first error, so only
`ew.Err` needs to be checked at the end. Never use bare `fmt.Fprintf` /
`fmt.Fprintln` without capturing the return value — `errcheck` will reject it.

When writing a single line with an early return, capture the error inline:

```go
_, err := fmt.Fprintln(w, "No builds found.")
return err
```

---

## 9. JSON output

Indented 2-space JSON emitted to stdout. Stable contract: fields may be added
but never renamed or removed without a major version bump.

```json
{
  "has_token": true,
  "token_type": "personal",
  "source": "env (BITRISE_TOKEN)",
  "path": "/Users/alice/.config/bitrise/auth.yaml"
}
```

`token_type` is omitted when no token is configured.

No ANSI codes. No diagnostic messages. Pure data.

### Errors in JSON mode

`--output json` only changes **stdout**. Errors are always plain text on
**stderr**, regardless of format. Cobra catches the `error` returned by `RunE`
and writes `Error: <message>` to stderr (`SilenceErrors: false`). There is no
JSON error envelope. A consumer can pipe stdout safely as JSON and read stderr
as human-readable error text.

---

## 10. Output-controlling flags

| Flag | Short | Default | Effect |
|------|-------|---------|--------|
| `--output` | `-o` | `human` | `human` or `json` |
| `--quiet` | `-q` | false | Suppresses non-error diagnostic messages on stderr |
| `--no-color` | | false | Forces ANSI-free output; `NO_COLOR` env also works |

All three are persistent (available on every subcommand) and resolved in
`persistentPreRun` before any handler runs.

---

## 11. Update notifications

`bitrise-cli` checks the GitHub Releases API of `bitrise-io/bitrise-cli` for a
newer version and, when the running build is behind, prints a one-line notice
to **stderr** after the command's own output:

```
A new release of bitrise-cli is available: 1.2.0 → 1.3.0
https://github.com/bitrise-io/bitrise-cli/releases/tag/v1.3.0
Upgrade: curl -fsSL https://app.bitrise.io/cli/install.sh | bash
```

It is a **diagnostic** (stderr, never stdout), so it never pollutes piped data
or JSON. The notice is styled with the `Warn` (version line) and `Dim` (hint)
styles and respects `--no-color`.

### Mechanics

- **Cached.** The last-check timestamp and latest known release are cached in
  `$XDG_CONFIG_HOME/bitrise/version-check.json` (0600). The network is hit at
  most once per 24h; every other invocation reads the cache and makes no call.
- **Best-effort.** Offline, rate-limited, or malformed responses produce no
  notice and never affect the command's exit status. A failed fetch still
  records the attempt so a transient outage backs off for the interval.
- **GitHub only.** The single network destination is GitHub's public API; no
  data is sent to Bitrise.

### When it is shown

A notice appears only when **all** of these hold (the policy lives in
`shouldCheckForUpdate`):

| Condition | Why |
|-----------|-----|
| `--output human` | JSON is a machine contract; stay silent |
| stderr is an interactive TTY | don't nag into pipes, files, or CI logs |
| not in CI (`CI` / `BITRISE_IO` unset) | CI runs are scripts, not readers |
| `BITRISE_CLI_NO_UPDATE_NOTIFIER` unset | explicit opt-out |
| not `--quiet` | `-q` suppresses stderr diagnostics |
| released build (clean `vX.Y.Z`) | dev builds have nothing to compare |
| a real subcommand (not `version`/`completion`) | `version` already shows it; completion output must stay clean |
