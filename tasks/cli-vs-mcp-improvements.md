# CLI improvements to widen the gap over MCP

Source: head-to-head benchmark of `bitrise-cli` vs the Bitrise MCP on a fixed
agent scenario (list apps → get app → trigger build on `main` workflow `test`,
wait for completion → tail of build log).

## Benchmark summary

| Metric | MCP | CLI v1 (orig) | CLI v2 (no redirect) | CLI v3 (quiet `--wait`) | CLI v4 (+ URL header, #1 fix) | **CLI v5 (404 fix)** |
|---|---|---|---|---|---|---|
| Total tokens (est.) | 55,987 | 50,418 | 50,853 | **36,418** | n/a | MCP: **~7,700** · CLI: ~11,200 |
| Tool/cmd uses | 16 | 15 | 30 | **6** | MCP: 8 · CLI: 4 | **MCP: 6 · CLI: 4** |
| Agent duration | 132.5s | 177.7s | 249.5s | **137.5s** | MCP: 143s · CLI: ~97s | **MCP: ~112s · CLI: ~96s** |
| Step 3 output lines | ~100 | ~627 | ~640 | **~17** | MCP: ~100 · CLI: 18 | **MCP: ~30 · CLI: 18** |
| Build runtime | ~80s | ~83s | ~96s | ~85s | MCP: 85s · CLI: 81s | MCP: 79s · CLI: 86s |

v5 token counts are measured from actual output sizes (chars ÷ 4). MCP polling required 2 get_build calls. Both runs were clean.

## Key takeaways v5

- **404 fix confirmed working.** `build trigger --wait` completed cleanly with no 404 error, no spurious re-trigger. The retry loop absorbed the provisioning window silently.
- **URL in header confirmed stable.** `→ https://…/build/<slug>` appeared on line 2 of stderr immediately after trigger — no regression.
- **CLI still fewer commands: 4 vs MCP's 6.** MCP needed trigger + 2 poll + 1 final get = 4 calls for step 3 alone. CLI used 1 command for the entire step.
- **Step 3 output gap closed:** CLI 18 lines / ~195 tokens (header + JSON) vs MCP ~30 lines / ~445 tokens (trigger + 2 polls). CLI still wins.
- **CLI is 46% more tokens overall (11,200 vs 7,700).** The entire gap is step 4 alone.
- **Step 4 is the single bottleneck.** CLI `build log` (full) cost ~7,100 tokens vs MCP tail (30 lines) ~540 tokens — a 13× gap. `build log --tail N` would close this completely, bringing CLI to ~4,100 tokens total vs MCP's ~7,700. CLI would win on tokens by ~47%.
- **Step 1 CLI is already cheaper than MCP.** CLI `app list` ~3,800 tokens vs MCP `list_apps` ~6,600 tokens — CLI wins despite pretty-printing, because it returns fewer fields per app. `--limit 20` default would save another ~1,500 tokens here.
- **Wall-clock CLI faster:** CLI ~96s vs MCP ~112s.
- **ANSI in log bytes persists on both sides.** MCP embeds `[32;1m` escape codes in JSON strings. CLI `build log` passes raw bytes with `[32;1m✓[0m` inline.

### Token breakdown (v5 measured)

| Step | CLI chars | CLI tokens | MCP chars | MCP tokens | Winner |
|---|---|---|---|---|---|
| Step 1: list | 15,333 | ~3,833 | 26,349 | ~6,587 | **CLI** (fewer fields) |
| Step 2: view | 257 | ~64 | 470 | ~117 | **CLI** |
| Step 3: trigger+wait | 782 | ~195 | 1,782 | ~445 | **CLI** (1 cmd vs 4 calls) |
| Step 4: log | 28,412 | ~7,103 | 2,155 | ~538 | **MCP** (tail vs full) |
| **Total** | **44,784** | **~11,195** | **30,756** | **~7,687** | MCP by 46% |
| After `--tail 30` | ~18,527 | ~4,632 | 30,756 | ~7,687 | **CLI by 40%** |

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

### ✅ PR #10 — Adaptive colors for light/dark terminals
`lipgloss.AdaptiveColor` replaces hardcoded palette entries so the CLI's own
formatting looks correct on both dark and light terminal backgrounds.
Structured output (tables, labels, status) is now clean for all TTY types.
Remaining ANSI gap: raw log bytes from the API still pass through unfiltered.

### ✅ PR #13 — Build URL in non-TTY watch header + 404 retry fix
Two fixes in one PR:

**URL in header:** Non-TTY `build watch` / `build trigger --wait` now shows
`→ https://…/<slug>` on the second line of stderr immediately after trigger.
Previously the URL only appeared in the final JSON record on stdout. Agents
can detach mid-run and resume with `build view <slug>` without waiting.

**404 retry fix:** `svc.Watch` retried `BuildLogManifest` only once after a
single interval (3s). The log manifest API returns 404 for ~5-15s while the
runner provisions. The old retry window wasn't enough — agents saw the error
and re-triggered the build, wasting a credit and 90+ seconds. Changed to
retry up to 10 times (~30s at the default 3s interval). Confirmed working in
v5 benchmark: no 404 error, no spurious re-trigger.

---

## P0 — token cost (biggest wins)

### 1. `--limit` sensible defaults on list commands
**Gap:** `--limit` exists on `app list` and `build list` but defaults to `0`
(server picks = 50 records). Agents asking "do I have access?" or "is there a
running build?" pay for 50 full records when 1 would suffice.

**Fix:** Change `IntVar(&limit, "limit", 0, ...)` to `20` on both commands.
Document `--limit 1` as the lean check pattern in the Long help text.

Effort: XS. Token win: ★★.

### 2. `build log --tail N` and `--since DURATION`
**Gap:** `build log` returns the full log — 513 lines / ~7,100 tokens in v5 vs
MCP's 30-line tail / ~540 tokens (`offset=-1`). This single step accounts for
**63% of all CLI tokens** and is the sole reason CLI trails MCP on token cost.
Agents pipe to `tail` externally but still pay network + tokens for the full
payload. Adding `--tail 30` would flip the leaderboard: CLI ~4,600 tokens vs
MCP ~7,700 — a 40% CLI win.

The service already uses `BuildLogManifest` with `afterTimestamp` internally
during `--wait`; `--tail N` is a thin wrapper on top.

Effort: S. Token win: ★★★.

### 3. Auto-strip ANSI from raw log bytes for non-TTY
**Gap:** The CLI's own formatting (tables, labels, status) is already clean:
lipgloss uses a `NewRenderer(w)` per writer, so pipes get ASCII automatically
(confirmed working in PR #10). The remaining gap is raw API log content from
`build log` and `--wait --verbose` — build steps emit ANSI color codes that
pass through unfiltered to non-TTY consumers. Confirmed in v5: `[32;1m✓[0m`
inline in both CLI and MCP outputs.

**Fix:** In `build log`, detect `!isatty(stdout)` (or check `NO_COLOR`) and
run bytes through a simple ANSI-stripping filter before writing. The
`--no-color` root flag already exists; wiring it to log passthrough is the
cleanest approach.

Effort: S. Token win: ★★.

## P1 — round-trip reduction (workflow density)

### 4. `build log --tail N --wait` (follow mode for finished builds)
One command instead of separate `build view` + `build log` chain. Mirrors how
`--wait` on trigger collapsed trigger + poll into a single call.

Effort: S. Token win: ★. Round-trip win: ★.

### 5. Field projection: `--output json --fields slug,status,status_text`
Already on the deferred list. Promote it. Lets agents cut response size 5–10×
on hot paths (`build view`, `app list`). This is the single feature MCP can't
match without per-tool support.

Effort: M. Token win: ★★★.

### 6. `app view --include workflows,branches,last-build`
MCP's `get_app` is shallow — same fields as a list row. One richer CLI call
avoids N follow-ups. Anything needed 90% of the time goes behind `--include`.

Effort: M. Token win: ★. Round-trip win: ★★.

### 7. Resolve `app view` flag/positional inconsistency
`build log` and `build view` accept the slug positionally and `--app` as a
flag. `app view` takes the slug positionally with no `--app` fallback (only
`BITRISE_APP_SLUG` env). This inconsistency costs one `--help` invocation.

Effort: XS. Token win: —. Round-trip win: ★ (fewer help lookups).

## P2 — JSON contract polish

### 8. Stable, narrow schemas per command (documented in `--help`)
`--output json` is already stable per CLAUDE.md. Agents would benefit from a
schema snippet right in `--help` — MCP has tool-defs visible to the model;
the CLI should match.

### 9. Consistent `status` field
Surface a single string `status` (`success|failed|aborted|running|queued`)
plus a numeric `status_code` if needed. Don't replicate MCP's split of
`status` (int) + `status_text` (string).

### 10. NDJSON `--watch` for status transitions
One JSON object per line, cheap to consume and emit.
```
{"event":"queued","ts":"..."}
{"event":"running","ts":"...","step":"git-clone"}
{"event":"finished","status":"success","duration_seconds":83}
```

## P3 — agent ergonomics

### 11. `--output json` as global default for non-TTY
When stdout isn't a TTY, default to JSON. Human format becomes opt-in via
`--output human`. Removes one flag from every agent invocation.

### 12. `--examples` flag or `bitrise-cli explain <command>`
Keep `--help` short; put a longer examples page behind `--examples` for
cold starts to eliminate diagnostic `--help` invocations.

### 13. `build trigger --wait --max-wait 5m`
Bound the wait so agents don't burn context on a stuck build. Exit non-zero
with a clear "still running, slug=X" record on timeout.

### 14. Document `config set output json` as the recommended agent setup
Already supported per CLAUDE.md. A one-liner agent setup snippet at the top
of `--help` means agents see it on cold start without reading docs.

---

# Effort vs impact

| Item | Effort | Token win | Round-trip win |
|---|---|---|---|
| ✅ `--wait` quiet by default (PR #9) | S | ★★★ | — |
| ✅ Adaptive colors (PR #10) | S | ★ | — |
| ✅ URL in non-TTY header + 404 fix (PR #13) | S | — | ★ |
| 1. `--limit` sensible defaults | XS | ★★ | — |
| 2. `build log --tail`/`--since` | S | ★★★ | — |
| 3. Auto-strip ANSI from log bytes | S | ★★ | — |
| 4. `build log --wait` follow mode | S | ★ | ★ |
| 5. Field projection `--fields=` | M | ★★★ | — |
| 6. `app view --include` | M | ★ | ★★ |
| 10. NDJSON `--watch` | M | ★★ | ★★ |

**Shipping #1, #2, and #3 closes the remaining confirmed friction gaps from the
v5 benchmark. Items #1 and #2 directly address the two areas where MCP still
has a structural edge (list payload size and log tail). All three are XS-or-S
effort.**
