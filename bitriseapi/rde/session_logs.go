package rde

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// LogChunk is one frame of a session log stream. HeartbeatMessage frames
// carry empty LogContent and exist only to keep the connection alive; callers
// skip them.
type LogChunk struct {
	LogContent       string `json:"logContent"`
	HeartbeatMessage bool   `json:"heartbeatMessage"`
}

// logStreamLine is one newline-delimited JSON frame emitted by the
// grpc-gateway ForwardResponseStream wrapper. A frame carries EITHER a result
// or an error: success frames look like {"result":{...}} and a mid-stream
// failure looks like {"error":{"code":int,"message":string,"details":[...]}}
// (a google.rpc.Status — code is a gRPC code, not HTTP).
type logStreamLine struct {
	Result LogChunk   `json:"result"`
	Error  *errorBody `json:"error"`
}

// maxLogLine caps the bufio.Scanner token size. Log lines can be long (stack
// traces, base64 blobs); the default 64KiB is too small, so allow up to 1MiB.
const maxLogLine = 1 << 20

// StreamSessionLogs opens the server-streaming log endpoint for one stage of a
// session and invokes fn for every non-empty content chunk, in order.
//
// Endpoint: GET /v1/workspaces/{workspaceId}/sessions/{sessionId}/logs/{stage}
// where stage is the numeric LogStage enum ("1"=warmup, "2"=main). The stream
// replays the stage log from the start, then continues live until the stage
// ends (EOF) or the connection drops. Heartbeat/empty frames are skipped.
//
// A mid-stream {"error":...} frame is surfaced as an error. A cancelled ctx
// (e.g. Ctrl-C) is treated as a clean end of stream and returns nil. Pre-stream
// failures come back as *APIError (e.g. 404 = "logs not available yet").
func (c *Client) StreamSessionLogs(ctx context.Context, workspaceID, sessionID, stage string, fn func(LogChunk) error) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace ID is required")
	}
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	p := wsPath(workspaceID, "/sessions/"+url.PathEscape(sessionID)+"/logs/"+url.PathEscape(stage))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+p, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := c.doStream(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLogLine)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var frame logStreamLine
		if err := json.Unmarshal(line, &frame); err != nil {
			return fmt.Errorf("decode log frame: %w", err)
		}
		if frame.Error != nil {
			return streamFrameError(frame.Error)
		}
		if frame.Result.HeartbeatMessage || frame.Result.LogContent == "" {
			continue
		}
		if err := fn(frame.Result); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		// A cancelled context surfaces as a read error on the body; treat it
		// as a clean end of stream so Ctrl-C exits 0.
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("read log stream: %w", err)
	}
	return nil
}

// streamFrameError renders a mid-stream {"error":...} frame as a plain error.
// Unlike a pre-stream HTTP failure, code here is a gRPC code (not an HTTP
// status), so this deliberately omits the "RDE API <status>" prefix and just
// surfaces the message (e.g. "log stream interrupted, please reconnect") plus
// any field violations.
func streamFrameError(e *errorBody) error {
	msg := e.Message
	if v := strings.Join(violationsFromDetails(e.Details), "; "); v != "" {
		if msg != "" {
			msg += ": " + v
		} else {
			msg = v
		}
	}
	if msg == "" {
		msg = "log stream error"
	}
	return errors.New(msg)
}
