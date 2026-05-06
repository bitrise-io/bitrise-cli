package bitriseapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const defaultBaseURL = "https://api.bitrise.io/v0.1"

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

// WithBaseURL overrides the default API base URL, useful for testing.
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = u }
}

// New creates a Client authenticated with the given token.
func New(token string, opts ...Option) *Client {
	c := &Client{
		baseURL:    defaultBaseURL,
		token:      token,
		httpClient: &http.Client{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// APIError represents a non-2xx response from the Bitrise API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("bitrise API %d: %s", e.StatusCode, e.Message)
}

type errorBody struct {
	Message string `json:"message"`
}

type envelope[T any] struct {
	Data T `json:"data"`
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
	resp, err := c.httpClient.Do(req)
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
		return nil, &APIError{StatusCode: resp.StatusCode, Message: e.Message}
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
