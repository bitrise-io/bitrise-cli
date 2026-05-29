# Plan: `bitrise-cli rde session logs`

Status: **not started** — design captured for a later session.

Goal: stream a session's provisioning/boot logs (the backend's
`GetSessionLogs` RPC), the one genuinely user-facing RDE capability the CLI
is missing. Useful for debugging a session stuck provisioning or that came
up `failed`.

## Backend facts (verified, the hard part)

- Endpoint: `GET /v1/workspaces/{workspace_id}/sessions/{session_id}/logs/{stage}/{type}`
  — grpc-gateway **server-streaming** (`backend/internal/api/session_logs.go`,
  proto `backend/proto/codespaces/v1/codespaces.proto` ~line 1709).
- Path enums are **numeric** (confirmed from the dashboard's
  `frontend/src/hooks/useSessionLogs.ts` `stageMap`/`typeMap`):
  - stage: `1` = warmup (runs once at creation), `2` = main (every start/restart)
  - type: `1` = stdout, `2` = stderr
- Body is **newline-delimited JSON frames** (grpc-gateway
  `ForwardResponseStream`, v2.28):
  - success: `{"result":{"logContent":"…","heartbeatMessage":bool}}`
  - mid-stream error: `{"error":{"code":<grpc code>,"message":"…","details":[…]}}`
    (a `google.rpc.Status` — note `code` is a gRPC code, NOT HTTP)
  - heartbeat frames carry empty `logContent` → skip them
- Pre-stream failures come back as ordinary HTTP non-2xx:
  - `404` = "logs not available yet for this stage" (stage hasn't produced
    output yet; the dashboard auto-retries)
  - `400` = bad stage/type, or "session has not been started yet, no logs available"
- The existing RDE client buffers the whole body in `do()` (`bitriseapi/rde/client.go`)
  → a non-buffering streaming path is required.
- Ctrl-C: the root command runs on a plain (non-signal) context, so the logs
  command must wrap `cmd.Context()` with `signal.NotifyContext(ctx, os.Interrupt)`
  itself. Precedent: `cmd/purr.go`. Cancelling the context ends the HTTP
  stream and the command exits 0.

## Files to add / change

1. **`bitriseapi/rde/client.go`**
   - Extract `parseAPIError(statusCode int, body []byte) *APIError` out of
     `do()` (pure refactor — `do` keeps identical behavior).
   - Add `doStream(req) (*http.Response, error)`: sets the standard
     auth/User-Agent/X-Request-Source headers; on non-2xx drains+closes the
     body and returns `*APIError` (via `parseAPIError`); on 2xx returns the
     live response (caller owns and MUST close `Body`).

2. **`bitriseapi/rde/session_logs.go`** (new)
   - `type LogChunk struct { LogContent string; HeartbeatMessage bool }`
     (json tags `logContent`/`heartbeatMessage`).
   - `type logStreamLine struct { Result LogChunk; Error *errorBody }`
     (reuses the existing `errorBody`).
   - `StreamSessionLogs(ctx, workspaceID, sessionID, stage, logType string, fn func(LogChunk) error) error`:
     build path with `wsPath` + `url.PathEscape(sessionID)`, call `doStream`,
     scan frames with a `bufio.Scanner` (raise buffer cap to ~1MB for long
     lines), skip heartbeats/empty content, surface `{"error"}` frames as an
     error, treat a cancelled `ctx` as clean EOF (return nil).

3. **`internal/rde/session_logs.go`** (new)
   - Friendly-name constants: `LogStageWarmup="warmup"`, `LogStageMain="main"`,
     `LogTypeStdout="stdout"`, `LogTypeStderr="stderr"`.
   - `StreamSessionLogs(ctx, ws, id, stage, logType string, fn func(string) error)`:
     map friendly names → numeric enum strings (`logStageToAPI`/`logTypeToAPI`,
     returning a clear error for unknown values), forward `chunk.LogContent`.

4. **`cmd/rde/session/logs.go`** (new)
   - `logs SESSION_ID`, flags `--stage` (default `main`) and `--type`
     (default `stdout`), with `RegisterFlagCompletionFunc` for both.
   - Validate `--stage`/`--type` against the known set in the cmd (mirrors how
     `notifications` validates `--order`) so errors are clear before the
     stderr header prints.
   - Reject `--output json` (mirror `view --watch`): the feed is plain text,
     not a single object.
   - Wrap `cmd.Context()` in `signal.NotifyContext(..., os.Interrupt)`.
   - Print a one-line stderr header unless `--quiet`
     ("Streaming main/stdout logs for session … — Ctrl-C to stop…").
   - Write each chunk verbatim to `cmd.OutOrStdout()` (no added newlines —
     reconstruct the exact log text, like the dashboard does).

5. **`cmd/rde/session/cmd.go`** — register `newLogsCmd()`.

6. **`README.md`** — add the `rde session logs` row to the command table.

7. **Tests**
   - `bitriseapi/rde`: httptest server emitting `{"result":…}` frames + a
     heartbeat → assert chunks delivered and heartbeat skipped; a `404` →
     `*APIError`; a mid-stream `{"error":…}` frame → error.
   - `internal/rde`: stage/type → numeric mapping; invalid → error.
   - `cmd/rde/session`: happy path streams to stdout; `--output json`
     rejected; bad `--stage` errors.

## Decisions (recommended defaults — confirm before building)

- **A. `--output json`** → **reject** (consistent with `view --watch`).
  Revisit with NDJSON if anyone needs machine-readable logs.
- **B. Default stage/type** → **main + stdout**.
- **C. Merge stdout+stderr?** → **no, single stream per call**. The API is
  per-(stage,type); merging needs two concurrent interleaved streams.
- **D. On `404` (not ready)** → **clear one-shot message** now
  ("logs not available yet for this stage; the session may still be
  provisioning — retry shortly"). Add `--wait`/retry later if wanted.

Note: there is no server-side "snapshot" mode — the endpoint streams until
the log source ends, so the command is inherently follow-like (returns when
the backend closes the stream or on Ctrl-C).

## Out of scope (separate gaps from the same analysis)

- `OpenRemoteAccess` wiring so `exec`/`upload`/`download` work without the
  dashboard having opened SSH first.
- Carrying `vnc_password` through (the shown VNC details are unusable without it).
