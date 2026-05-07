# CLI improvements to widen the gap over MCP

Source: head-to-head benchmark of `bitrise-cli` vs the Bitrise MCP on a fixed
agent scenario (list apps → get app → trigger build on `main` workflow `test`,
wait for completion → tail of build log).

## Benchmark summary

| Metric | MCP | CLI v1 (orig) | CLI v2 (no redirect) | CLI v3 (quiet `--wait`) | **CLI v4 (+ URL header, #1 fix)** |
|---|---|---|---|---|---|
| Total tokens | 55,987 | 50,418 | 50,853 | **36,418** | n/a (not telemetry-measured) |
| Tool/cmd uses | 16 | 15 | 30 | **6** | MCP: 8 · CLI: 4 (clean) |
| Agent duration | 132.5s | 177.7s | 249.5s | **137.5s** | MCP: 143s · CLI: ~97s (clean) |
| Step 3 output lines | ~100 | ~627 | ~640 | **~17** | MCP: ~100 · CLI: **18** |
| Build runtime | ~80s | ~83s | ~96s | ~85s | MCP: 85s · CLI: 81s |

v4 CLI run was conducted manually (not via telemetry). CLI clean-run duration excludes a spurious build triggered by a `--wait` 404 bug discovered during the run (see below).

## Key takeaways v4

- **CLI still ahead on commands and step-3 noise.** 4 CLI commands vs 8 MCP calls; 18 output lines vs ~100 for step 3.
- **CLI faster on wall-clock (~97s vs 143s)** when the 404 bug doesn't trigger.
- **New bug found: `build trigger --wait` can 404 immediately after trigger.** The log manifest isn't available for a few seconds after trigger; the retry logic in `svc.Watch` failed to handle this, causing an immediate error. The build still ran, but the agent had to check manually and re-trigger. This inflated the observed CLI run to 263s total (two builds). Fixed retry behavior is needed.
- **URL-in-header confirmed working.** The non-TTY stderr now shows `→ https://…/build/<slug>` immediately — the slug was available without waiting for the final JSON record.
- **Both setups still have ANSI in log output.** MCP embeds `[32;1m` escape codes in JSON strings. CLI `build log` pipes raw bytes with `[32;1m✓[0m` inline. Neither is clean for non-TTY consumers; no step forward here yet.
- **CLI `build log` step 4 cost: 516 lines vs MCP's ~30.** MCP used `offset=-1` for tail-only. CLI has no `--tail` flag yet, so agents must pipe through `tail` externally and still pay the full network transfer.
- **`app list` returns 50 records** on both MCP and CLI (CLI default `--limit 0` = server default). Both wasteful for "do I have access?" checks.

## Key takeaways (historical)

- **CLI v3 won decisively** — 36k tokens, 6 commands, on-par wall-clock.
  `--wait` quiet-by-default delivered a 97% reduction in step-3 output and
  −35% total tokens vs MCP.
- **MCP friction:** numeric `status` codes (0/1), `list_apps` returns 50 full
  records with no lean preview, `get_build_log` embeds raw ANSI in JSON
  strings, no native wait primitive — 7 poll calls are hand-rolled.

---

# Improvements (ordered by agent-impact)

## Shipped

### ✅ PR #9 — `--wait` quiet by default; `--verbose` to restore streaming
Step-3 output dropped from ~625 lines to ~17 (2-line header + 15-line JSON).
Total tokens: ~50k → ~36k. Largest single efficiency win.

### 🐛 Bug: `build trigger --wait` 404s immediately after trigger
`svc.Watch` calls `BuildLogManifest` on the first poll. The log manifest
returns 404 for a few seconds after trigger while the runner provisions.
The retry-on-404 logic in `service.go` (lines 224-240) exhausted before
the log became available, surfacing the error to the caller instead of
waiting. An agent receiving this error re-triggers the build, wasting a
build credit and 90+ extra seconds.

**Fix:** Increase the initial 404 retry count / backoff in `service.go`
so the first-available-log window (typically 5-10s) is covered.

### ✅ PR #10 — Adaptive colors for light/dark terminals
`lipgloss.AdaptiveColor` replaces hardcoded palette entries so the CLI's own
formatting looks correct on both dark and light terminal backgrounds.
Structured output (tables, labels, status) is now clean for all TTY types.
Remaining ANSI gap: raw log bytes from the API still pass through unfiltered.

---

## P0 — token cost (biggest wins)

### 1. Build URL in non-TTY watch header
**Gap:** In TTY mode the TUI status bar shows `→ <url>` immediately — the
build slug is right there. In non-TTY mode (piped output, agent harness) the
header only prints `Watching build #N — workflow 'X' on branch 'Y'`; the URL
never appears until the final record.

**Fix:** Append `\n→ <BuildURL>` to `buildWatchHeader()` in
[cmd/build/utils.go](cmd/build/utils.go) when `b.BuildURL != ""`. One line.
This also applies to `build watch` since it shares the same header path.

Effort: XS. Token win: —. Round-trip win: ★ (agents can detach mid-run and
resume with `build view <slug>`).

### 2. `--limit` sensible defaults on list commands
**Gap:** `--limit` exists on `app list` and `build list` but defaults to `0`
(server picks = 50 records). Agents asking "do I have access?" or "is there a
running build?" pay for 50 full records when 1 would suffice.

**Fix:** Change `IntVar(&limit, "limit", 0, ...)` to `20` on both commands.
Document `--limit 1` as the lean check pattern in the Long help text.

Effort: XS. Token win: ★★.

### 3. `build log --tail N` and `--since DURATION`
**Gap:** `build log` returns the full log — agents pipe to `tail` but still
pay network + tokens for the full payload. MCP exposes `offset=-1` for
tail-only; the CLI should match.

The service already uses `BuildLogManifest` with `afterTimestamp` internally
during `--wait`; `--tail N` is a thin wrapper on top.

Effort: S. Token win: ★★★.

### 4. Auto-strip ANSI from raw log bytes for non-TTY
**Gap:** The CLI's own formatting (tables, labels, status) is already clean:
lipgloss uses a `NewRenderer(w)` per writer, so pipes get ASCII automatically
(confirmed working in PR #10). The remaining gap is raw API log content from
`build log` and `--wait --verbose` — build steps emit ANSI color codes that
pass through unfiltered to non-TTY consumers.

**Fix:** In `build log`, detect `!isatty(stdout)` (or check `NO_COLOR`) and
run bytes through a simple ANSI-stripping filter before writing. The
`--no-color` root flag already exists; wiring it to log passthrough is the
cleanest approach.

Effort: S. Token win: ★★.

## P1 — round-trip reduction (workflow density)

### 5. `build log --tail N --wait` (follow mode for finished builds)
One command instead of separate `build view` + `build log` chain. Mirrors how
`--wait` on trigger collapsed trigger + poll into a single call.

Effort: S. Token win: ★. Round-trip win: ★.

### 6. Field projection: `--output json --fields slug,status,status_text`
Already on the deferred list. Promote it. Lets agents cut response size 5–10×
on hot paths (`build view`, `app list`). This is the single feature MCP can't
match without per-tool support.

Effort: M. Token win: ★★★.

### 7. `app view --include workflows,branches,last-build`
MCP's `get_app` is shallow — same fields as a list row. One richer CLI call
avoids N follow-ups. Anything needed 90% of the time goes behind `--include`.

Effort: M. Token win: ★. Round-trip win: ★★.

### 8. Resolve `app view` flag/positional inconsistency
`build log` and `build view` accept the slug positionally and `--app` as a
flag. `app view` takes the slug positionally with no `--app` fallback (only
`BITRISE_APP_SLUG` env). This inconsistency costs one `--help` invocation.

Effort: XS. Token win: —. Round-trip win: ★ (fewer help lookups).

## P2 — JSON contract polish

### 9. Stable, narrow schemas per command (documented in `--help`)
`--output json` is already stable per CLAUDE.md. Agents would benefit from a
schema snippet right in `--help` — MCP has tool-defs visible to the model;
the CLI should match.

### 10. Consistent `status` field
Surface a single string `status` (`success|failed|aborted|running|queued`)
plus a numeric `status_code` if needed. Don't replicate MCP's split of
`status` (int) + `status_text` (string).

### 11. NDJSON `--watch` for status transitions
One JSON object per line, cheap to consume and emit.
```
{"event":"queued","ts":"..."}
{"event":"running","ts":"...","step":"git-clone"}
{"event":"finished","status":"success","duration_seconds":83}
```

## P3 — agent ergonomics

### 12. `--output json` as global default for non-TTY
When stdout isn't a TTY, default to JSON. Human format becomes opt-in via
`--output human`. Removes one flag from every agent invocation.

### 13. `--examples` flag or `bitrise-cli explain <command>`
Keep `--help` short; put a longer examples page behind `--examples` for
cold starts to eliminate diagnostic `--help` invocations.

### 14. `build trigger --wait --max-wait 5m`
Bound the wait so agents don't burn context on a stuck build. Exit non-zero
with a clear "still running, slug=X" record on timeout.

### 15. Document `config set output json` as the recommended agent setup
Already supported per CLAUDE.md. A one-liner agent setup snippet at the top
of `--help` means agents see it on cold start without reading docs.

---

# Effort vs impact

| Item | Effort | Token win | Round-trip win |
|---|---|---|---|
| ✅ `--wait` quiet by default (PR #9) | S | ★★★ | — |
| ✅ Adaptive colors (PR #10) | S | ★ | — |
| 1. URL in non-TTY header | XS | — | ★ |
| 2. `--limit` sensible defaults | XS | ★★ | — |
| 3. `build log --tail`/`--since` | S | ★★★ | — |
| 4. Auto-strip ANSI from log bytes | S | ★★ | — |
| 5. `build log --wait` follow mode | S | ★ | ★ |
| 6. Field projection `--fields=` | M | ★★★ | — |
| 7. `app view --include` | M | ★ | ★★ |
| 11. NDJSON `--watch` | M | ★★ | ★★ |

**Shipping #1, #2, #3, and #4 closes every confirmed friction gap from the
v3 benchmark. All four are XS-or-S effort. Together they eliminate the last
categories where MCP still has a structural edge.**
