package bitriseapi

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

// Client is an authenticated HTTP client for the Bitrise API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the default HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// New creates a Client authenticated with the given token and base URL.
func New(baseURL, token string, opts ...Option) *Client {
	c := &Client{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// APIError represents a non-2xx response from the Bitrise API. Message is
// the human-readable text extracted from a known JSON error field; Body is
// the raw response body, surfaced only when no structured field was found
// so unexpected error shapes still tell the user something.
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("bitrise API %d: %s", e.StatusCode, e.Message)
	}
	if e.Body != "" {
		return fmt.Sprintf("bitrise API %d: %s", e.StatusCode, truncate(e.Body, 500))
	}
	return fmt.Sprintf("bitrise API %d", e.StatusCode)
}

// errorBody covers the common JSON error shapes the Bitrise services
// return: {"message":...}, {"error_msg":...}, {"error":...}, and
// {"errors":[...]}.
type errorBody struct {
	Message  string   `json:"message"`
	ErrorMsg string   `json:"error_msg"`
	Error    string   `json:"error"`
	Errors   []string `json:"errors"`
}

// pick returns the first non-empty field of e, joining Errors with "; ".
func (e errorBody) pick() string {
	if e.Message != "" {
		return e.Message
	}
	if e.ErrorMsg != "" {
		return e.ErrorMsg
	}
	if e.Error != "" {
		return e.Error
	}
	if len(e.Errors) > 0 {
		return strings.Join(e.Errors, "; ")
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

type envelope[T any] struct {
	Data T `json:"data"`
}

type pagedEnvelope[T any] struct {
	Data   []T    `json:"data"`
	Paging Paging `json:"paging"`
}

func (c *Client) newRequest(ctx context.Context, path string, params url.Values) (*http.Request, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	if len(params) > 0 {
		u.RawQuery = params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.httpClient.Do(req) //nolint:gosec // URL is built from configured base + internal paths, not user-tainted input
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
		msg := e.pick()
		apiErr := &APIError{StatusCode: resp.StatusCode, Message: msg}
		if msg == "" {
			// No structured field — keep the raw body so the user has
			// something concrete to see (e.g. an unmarshalable Rails 500
			// HTML page or an undocumented error shape).
			apiErr.Body = strings.TrimSpace(string(body))
		}
		return nil, apiErr
	}
	return body, nil
}

// get performs a GET request and decodes the "data" field into T.
func get[T any](ctx context.Context, c *Client, path string, params url.Values) (T, error) {
	var zero T

	req, err := c.newRequest(ctx, path, params)
	if err != nil {
		return zero, err
	}

	body, err := c.do(req)
	if err != nil {
		return zero, err
	}

	var env envelope[T]
	if err := json.Unmarshal(body, &env); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}
	return env.Data, nil
}

// getPage performs a GET request and decodes a paginated list response,
// returning a Page[T] with the items and the paging metadata.
func getPage[T any](ctx context.Context, c *Client, path string, params url.Values) (Page[T], error) {
	req, err := c.newRequest(ctx, path, params)
	if err != nil {
		return Page[T]{}, err
	}

	body, err := c.do(req)
	if err != nil {
		return Page[T]{}, err
	}

	var env pagedEnvelope[T]
	if err := json.Unmarshal(body, &env); err != nil {
		return Page[T]{}, fmt.Errorf("decode response: %w", err)
	}
	return Page[T]{Items: env.Data, Paging: env.Paging}, nil
}

// postDecode marshals body as JSON, POSTs to path, and decodes the response
// directly into Resp. Used for endpoints whose responses don't use the
// standard {"data": ...} envelope (e.g. POST /apps/{slug}/builds).
func postDecode[Req, Resp any](ctx context.Context, c *Client, path string, body Req) (Resp, error) {
	var zero Resp

	data, err := json.Marshal(body)
	if err != nil {
		return zero, fmt.Errorf("marshal request: %w", err)
	}
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return zero, fmt.Errorf("parse URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(data))
	if err != nil {
		return zero, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	respBody, err := c.do(req)
	if err != nil {
		return zero, err
	}
	var resp Resp
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}
	return resp, nil
}
