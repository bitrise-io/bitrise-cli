// Package rde is the HTTP client for the Bitrise Remote Dev Environments
// API (https://api.bitrise.io/rde).
//
// This is intentionally a sibling of bitriseapi/ and not merged in: the RDE
// service uses a Bearer authorization header, lives under a different base
// URL, and emits camelCase JSON. Keeping it separate avoids muddying the
// existing client.
//
// Wire-format DTOs in this package match the backend's swagger output
// (lowerCamelCase property names from grpc-gateway). The CLI-facing layer
// in internal/rde converts them into the stable snake_case `--output json`
// shape via fromAPI mappers.
package rde

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// UserAgent is sent on every RDE request. cmd/root.go overrides it at
// init time to include the binary's version ("bitrise-cli/X.Y.Z"). The
// backend uses this to attribute traffic to CLI vs MCP vs other clients.
var UserAgent = "bitrise-cli"

// RequestSource is the value of the X-Request-Source header on every
// RDE request. Mirrors the MCP's "X-Request-Source: mcp" pattern so the
// backend can distinguish CLI from MCP traffic without parsing the UA.
const RequestSource = "cli"

// Client is an authenticated HTTP client for the RDE API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the default HTTP client (useful for tests).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// New creates a Client authenticated with the given token and base URL.
// baseURL should be the RDE API root (e.g. https://api.bitrise.io/rde) —
// resource paths are appended verbatim.
func New(baseURL, token string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// APIError represents a non-2xx response from the RDE API. Message is the
// human-readable text extracted from the {"message": "..."} field RDE uses
// universally; Violations holds field-level validation messages pulled from
// the gRPC error details (details[].fieldViolations[]), which carry the
// actionable "why" for 400s (e.g. "missing required input: BUILD_TOKEN");
// Body is the raw response body, surfaced only when no structured field
// was found. Refusal is set when the backend refused the request rather
// than rejecting it — a full pool, a workspace out of CPUs — and the error
// then renders as the refusal's message plus the caller's own sessions.
type APIError struct {
	StatusCode int
	Message    string
	Violations []string
	Body       string
	Refusal    *WarmPoolRefusal
}

// WarmPoolRefusal is the codespaces.v1.WarmPoolRefusal detail of a
// RESOURCE_EXHAUSTED (HTTP 429) response: why a claim, restore or start
// was refused, the numbers, the caller's own sessions in use from the pool
// and the next steps. The status message is written to be read as is.
type WarmPoolRefusal struct {
	// Reason is "POOL_FULL" or "CPU_QUOTA_EXHAUSTED".
	Reason string `json:"reason"`
	// Action is "claim", "restore" or "create".
	Action       string `json:"action"`
	WarmPoolID   string `json:"warmPoolId"`
	WarmPoolName string `json:"warmPoolName"`
	// InUse and MaxSessions are the pool's numbers (POOL_FULL).
	InUse       int `json:"inUse"`
	MaxSessions int `json:"maxSessions"`
	// Family, UsedVCPU and QuotaVCPU are the quota's (CPU_QUOTA_EXHAUSTED);
	// UsedVCPU is nil when the backend does not know it.
	Family       string                   `json:"family"`
	UsedVCPU     *int                     `json:"usedVcpu"`
	QuotaVCPU    int                      `json:"quotaVcpu"`
	YourSessions []WarmPoolRefusalSession `json:"yourSessions"`
	NextSteps    []string                 `json:"nextSteps"`
}

// WarmPoolRefusalSession is one of the caller's own sessions in use from
// the pool a refusal names.
type WarmPoolRefusalSession struct {
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
	Status    string `json:"status"`
}

// warmPoolRefusalType is the @type of the refusal detail.
const warmPoolRefusalType = "type.googleapis.com/codespaces.v1.WarmPoolRefusal"

func (e *APIError) Error() string {
	if e.Refusal != nil {
		return e.refusalText()
	}
	msg := e.Message
	if msg == "" {
		msg = truncate(e.Body, 500)
	}
	if detail := strings.Join(e.Violations, "; "); detail != "" {
		if msg != "" {
			msg = msg + ": " + detail
		} else {
			msg = detail
		}
	}
	if msg != "" {
		return fmt.Sprintf("RDE API %d: %s", e.StatusCode, msg)
	}
	return fmt.Sprintf("RDE API %d", e.StatusCode)
}

// refusalText is the refusal as plain text: the message (what happened,
// your sessions, what to do, the admin — already in the order a person
// reads it), then the caller's own sessions with their IDs, which the
// message names without. No "error" wording: the pool is doing what it was
// set up to do.
func (e *APIError) refusalText() string {
	var b strings.Builder
	b.WriteString(e.Message)
	if b.Len() == 0 {
		b.WriteString("Refused: " + e.Refusal.Reason)
	}
	if len(e.Refusal.YourSessions) > 0 {
		b.WriteString("\nYour sessions in this pool:")
		for _, s := range e.Refusal.YourSessions {
			fmt.Fprintf(&b, "\n  %s (%s)  %s", s.Name, s.Status, s.SessionID)
		}
	}
	return b.String()
}

// errorBody covers the JSON error envelope RDE uses: a gRPC-gateway status
// ({"code": int, "message": string, "details": [...]}). Each details entry
// is a google.rpc.* message; BadRequest entries carry fieldViolations whose
// descriptions explain validation failures.
type errorBody struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Details []errorDetail `json:"details"`
}

// errorDetail is one entry of the gRPC status details array. The
// fieldViolations of google.rpc.BadRequest and the fields of
// codespaces.v1.WarmPoolRefusal are consumed; other detail types unmarshal
// with both empty and are ignored.
type errorDetail struct {
	Type            string           `json:"@type"`
	FieldViolations []fieldViolation `json:"fieldViolations"`
	WarmPoolRefusal
}

// fieldViolation is a single google.rpc.BadRequest.FieldViolation.
type fieldViolation struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// wsPath builds a workspace-scoped path under /v1/workspaces/{wsID}/...
// Used by every RDE resource except /v1/me and /v1/saved-inputs.
func wsPath(wsID, p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "/v1/workspaces/" + url.PathEscape(wsID) + p
}

// userPath builds a non-workspace-scoped path (currently /v1/me and
// /v1/saved-inputs/...).
func userPath(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "/v1" + p
}

// do executes req and returns the response body on 2xx. Non-2xx responses
// are returned as *APIError.
func (c *Client) do(req *http.Request) ([]byte, error) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("X-Request-Source", RequestSource)

	resp, err := c.httpClient.Do(req) //nolint:gosec // URL built from configured base + internal paths
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, body)
	}
	return body, nil
}

// parseAPIError turns a non-2xx (statusCode, body) into an *APIError. Shared
// by do() and doStream() so the buffering and streaming paths surface errors
// identically.
func parseAPIError(statusCode int, body []byte) *APIError {
	var e errorBody
	_ = json.Unmarshal(body, &e)
	// A streaming endpoint (session logs) wraps the same status in
	// {"error":{...}}, like its mid-stream error frames.
	var wrapped struct {
		Error *errorBody `json:"error"`
	}
	if e.Message == "" && json.Unmarshal(body, &wrapped) == nil && wrapped.Error != nil {
		e = *wrapped.Error
	}
	apiErr := &APIError{
		StatusCode: statusCode,
		Message:    e.Message,
		Violations: violationsFromDetails(e.Details),
		Refusal:    refusalFromDetails(e.Details),
	}
	// Fall back to the raw body only when neither a message nor any field
	// violation gave us something human-readable.
	if e.Message == "" && len(apiErr.Violations) == 0 {
		apiErr.Body = strings.TrimSpace(string(body))
	}
	return apiErr
}

// violationsFromDetails pulls the human-readable field-violation strings out
// of a gRPC status' details array (preferring the description, falling back to
// the field name). Shared by HTTP-level and mid-stream error rendering.
func violationsFromDetails(details []errorDetail) []string {
	var out []string
	for _, d := range details {
		for _, v := range d.FieldViolations {
			switch {
			case v.Description != "":
				out = append(out, v.Description)
			case v.Field != "":
				out = append(out, v.Field)
			}
		}
	}
	return out
}

// refusalFromDetails picks the codespaces.v1.WarmPoolRefusal detail out of
// a gRPC status' details array; nil when there is none.
func refusalFromDetails(details []errorDetail) *WarmPoolRefusal {
	for _, d := range details {
		if d.Type == warmPoolRefusalType && d.Reason != "" {
			r := d.WarmPoolRefusal
			return &r
		}
	}
	return nil
}

// doStream executes req and, on a 2xx response, returns the live *http.Response
// without buffering the body — the caller owns it and MUST close resp.Body.
// Used by streaming endpoints (e.g. session logs) where do()'s ReadAll would
// defeat the point. Non-2xx responses are drained, closed, and returned as an
// *APIError, matching do().
func (c *Client) doStream(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("X-Request-Source", RequestSource)

	resp, err := c.httpClient.Do(req) //nolint:gosec // URL built from configured base + internal paths
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, parseAPIError(resp.StatusCode, body)
	}
	return resp, nil
}

// getJSON performs a GET against path and decodes the response into out.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	body, err := c.do(req)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// sendJSON marshals reqBody as JSON, sends it via method to path, and
// decodes the response into out (skipped when out is nil).
func (c *Client) sendJSON(ctx context.Context, method, path string, reqBody, out any) error {
	var r io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	body, err := c.do(req)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// del performs a DELETE on path; responses are discarded.
func (c *Client) del(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	_, err = c.do(req)
	return err
}
