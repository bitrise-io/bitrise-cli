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
// universally; Body is the raw response body, surfaced only when no
// structured field was found.
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("RDE API %d: %s", e.StatusCode, e.Message)
	}
	if e.Body != "" {
		return fmt.Sprintf("RDE API %d: %s", e.StatusCode, truncate(e.Body, 500))
	}
	return fmt.Sprintf("RDE API %d", e.StatusCode)
}

// errorBody covers the JSON error envelope RDE uses
// ({"code": int, "message": string, "details": [...]}).
type errorBody struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details"`
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
		var e errorBody
		_ = json.Unmarshal(body, &e)
		apiErr := &APIError{StatusCode: resp.StatusCode, Message: e.Message}
		if e.Message == "" {
			apiErr.Body = strings.TrimSpace(string(body))
		}
		return nil, apiErr
	}
	return body, nil
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
